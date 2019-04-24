package main

import (
	"bytes"
	"encoding/json"
	"errors"
	//"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fogleman/gg"
	"github.com/fsnotify/fsnotify"
	"github.com/golang/geo/r2"
	"github.com/golang/geo/s2"
	"github.com/paulmach/go.geojson"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Index struct {
	Collections map[string]*Collection
	mutex       sync.RWMutex
	PublicPath  *url.URL
	watcher     *fsnotify.Watcher
}

type CollectionMetadata struct {
	Name         string
	Path         string
	LastModified time.Time
}

type Collection struct {
	metadata    CollectionMetadata
	dataFile    *os.File // temporary file, will be deleted
	offset      []int64  // offset into dataFile
	bbox        []s2.Rect
	webMercator []r2.Point
	id          []string
	byID        map[string]int // "W77" -> 3 if Features[3].ID == "W77"
}

func (c *Collection) Close() {
	if c.dataFile != nil {
		c.dataFile.Close()
		os.Remove(c.dataFile.Name())
	}
}

var (
	lastDataLoad = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "miniwfs_data_load_timestamp",
		Help: "Timestamp when data was last loaded, in seconds since the Unix epoch.",
	})
	numDataLoads = promauto.NewCounter(prometheus.CounterOpts{
		Name: "miniwfs_data_loads_total",
		Help: "Total number of data loads.",
	})
	numDataLoadErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "miniwfs_data_load_errors_total",
		Help: "Total number of errors when loading data.",
	})
	collectionFeaturesCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "miniwfs_collection_features",
		Help: "Number of features per collection.",
	},
		[]string{"collection"})
	collectionTimestamp = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "miniwfs_collection_timestamp",
		Help: "Timestamp of the collection, in seconds since the Unix epoch.",
	},
		[]string{"collection", "stage"})
)

func MakeIndex(collections map[string]string, publicPath *url.URL) (*Index, error) {
	index := &Index{
		Collections: make(map[string]*Collection),
		PublicPath:  publicPath,
	}

	if watcher, err := fsnotify.NewWatcher(); err == nil {
		index.watcher = watcher
	} else {
		return nil, err
	}

	go index.watchFiles()
	for name, path := range collections {
		var t0 time.Time // The zero value of type Time is January 1, year 1.
		coll, err := readCollection(name, path, t0)
		if err != nil {
			return nil, err
		}
		index.Collections[name] = coll
	}

	for _, c := range index.Collections {
		dirPath := filepath.Dir(c.metadata.Path)
		if err := index.watcher.Add(dirPath); err != nil {
			return nil, err
		}
	}

	return index, nil
}

func (index *Index) Close() {
	index.mutex.Lock()
	defer index.mutex.Unlock()
	for _, c := range index.Collections {
		c.Close()
		index.watcher.Remove(filepath.Dir(c.metadata.Path))
	}
	index.Collections = make(map[string]*Collection)
}

func (index *Index) GetCollections() []CollectionMetadata {
	index.mutex.RLock()
	defer index.mutex.RUnlock()

	md := make([]CollectionMetadata, 0, len(index.Collections))
	for _, coll := range index.Collections {
		md = append(md, coll.metadata)
	}
	sort.Slice(md, func(i, j int) bool { return md[i].Name < md[j].Name })
	return md
}

func (index *Index) GetItem(collection string, id string) (*geojson.Feature, error) {
	index.mutex.RLock()
	defer index.mutex.RUnlock()

	coll := index.Collections[collection]
	if coll == nil {
		return nil, nil
	}

	i, ok := coll.byID[id]
	if !ok {
		return nil, nil
	}

	offset := coll.offset[i]
	jsonLen := int(coll.offset[i+1] - offset - 2)
	b := make([]byte, jsonLen)
	if _, err := coll.dataFile.ReadAt(b, offset); err != nil {
		return nil, err
	}

	var result geojson.Feature
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// We take both startID and startIndex to be more resilient when our
// data changes while a client is iterating over paged results. If
// startID is a known ID, we start the iteration there; otherwise, we
// start the iteration at the feature whose index is startIndex.
//
// If the collection has not been modified since time ifModifiedSince,
// we return error NotModified (unless ifModifiedSince.IsZero() is true).
func (index *Index) GetItems(collection string, startID string, startIndex int, limit int, bbox s2.Rect,
	ifModifiedSince time.Time, ifUnmodifiedSince time.Time, out io.Writer) (CollectionMetadata, error) {
	// We intentionally return CollectionMetadata and not *CollectionMetadata
	// so that the metadata gets copied before unlocking the reader mutex.
	// Otherwise, the metadata content could change after returning from
	// this function. The same problem does not occur with *WFSFeatureCollection
	// because that is freshly allocated from scratch, and its members point to
	// objects that are not overwritten.
	index.mutex.RLock()
	defer index.mutex.RUnlock()

	coll := index.Collections[collection]
	if coll == nil {
		return CollectionMetadata{}, NotFound
	}

	lastModified := coll.metadata.LastModified.Round(time.Second).UTC()
	if !ifUnmodifiedSince.IsZero() && lastModified.After(ifUnmodifiedSince.Round(time.Second).UTC()) {
		return coll.metadata, Modified
	}
	if !ifModifiedSince.IsZero() && !lastModified.After(ifModifiedSince.Round(time.Second).UTC()) {
		return coll.metadata, NotModified
	}

	if limit < 1 {
		limit = 1
	} else if limit > MaxLimit {
		limit = MaxLimit
	}

	if len(startID) > 0 {
		if i, ok := coll.byID[startID]; ok {
			startIndex = i
		}
	}

	if startIndex < 0 {
		startIndex = 0
	}

	if _, err := out.Write([]byte(`{"type":"FeatureCollection","features":[`)); err != nil {
		return CollectionMetadata{}, err
	}

	// If we had more data, we could compute s2 cell coverages and only
	// check the intersection for features inside the coverage area.
	// But we operate on a few thousand features, so let's keep things simple
	// for the time being.
	bounds := s2.EmptyRect()
	var nextID string
	var nextIndex int
	skip := startIndex
	numFeatures := 0
	buffer := make([]byte, 0, 50*1024)
	for i, featureBounds := range coll.bbox {
		if !bbox.Intersects(featureBounds) {
			continue
		}

		if numFeatures >= limit {
			nextID = coll.id[i]
			nextIndex = i
			break
		}
		if skip > 0 {
			skip = skip - 1
			continue
		}

		if numFeatures > 0 {
			if _, err := out.Write([]byte{','}); err != nil {
				return CollectionMetadata{}, err
			}
		}

		b := buffer
		jsonLen := int(coll.offset[i+1] - coll.offset[i] - 2)
		if jsonLen > cap(b) {
			b = make([]byte, 0, jsonLen)
		}
		if _, err := coll.dataFile.ReadAt(b[0:jsonLen], coll.offset[i]); err != nil {
			return CollectionMetadata{}, err
		}
		if _, err := out.Write(b[0:jsonLen]); err != nil {
			return CollectionMetadata{}, err
		}

		numFeatures += 1
		bounds = bounds.Union(featureBounds)
	}

	if _, err := out.Write([]byte(`],`)); err != nil {
		return CollectionMetadata{}, err
	}

	type Footer struct {
		Links       []*WFSLink `json:"links,omitempty"`
		BoundingBox []float64  `json:"bbox,omitempty"`
	}
	var footer Footer

	pathPrefix := index.PublicPath.String()
	selfLink := &WFSLink{
		Rel:   "self",
		Title: "self",
		Type:  "application/geo+json",
	}

	selfLink.Href = FormatItemsURL(pathPrefix, collection, startID, startIndex, limit, bbox)
	footer.Links = append(footer.Links, selfLink)
	footer.BoundingBox = EncodeBbox(bounds)

	if nextIndex > 0 {
		nextLink := &WFSLink{
			Rel:   "next",
			Title: "next",
			Type:  "application/geo+json",
		}
		nextLink.Href = FormatItemsURL(pathPrefix, collection, nextID, nextIndex, limit, bbox)
		footer.Links = append(footer.Links, nextLink)
	}

	encodedFooter, err := json.Marshal(footer)
	if err != nil {
		return CollectionMetadata{}, err
	}
	if _, err := out.Write(encodedFooter[1:]); err != nil {
		return CollectionMetadata{}, err
	}

	return coll.metadata, nil
}

func (index *Index) watchFiles() {
	// We watch the local file system for changes so we quickly catch modifications.
	// Additionally, we check once per minute if the files have changed because
	// file system watching has not been very reliable in our experience.
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			for _, md := range index.GetCollections() {
				index.reloadIfChanged(md)
			}

		case event, ok := <-index.watcher.Events:
			log.Printf("Watcher event: %v\n", event)
			if !ok {
				return
			}
			if event.Op&fsnotify.Remove == fsnotify.Remove {
				return
			}
			path := event.Name
			md := index.getCollectionMetadata(path)
			if md != nil {
				index.reloadIfChanged(*md)
			}
		}
	}
}

func (index *Index) GetTile(collection string, zoom int, x int, y int) ([]byte, CollectionMetadata, error) {
	index.mutex.RLock()
	defer index.mutex.RUnlock()

	coll := index.Collections[collection]
	if coll == nil {
		return nil, CollectionMetadata{}, NotFound
	}

	scale := 1 << uint8(zoom)
	tileBounds := getTileBounds(zoom, x, y)
	tileOrigin := r2.Point{X: float64(x) * 256.0 / float64(scale),
		Y: float64(y) * 256.0 / float64(scale)}

	dc := gg.NewContext(256, 256)
	dc.SetRGBA255(255, 255, 255, 0)
	dc.Clear()
	empty := true
	for i, featureBounds := range coll.bbox {
		if !tileBounds.Intersects(featureBounds) {
			continue
		}
		p := coll.webMercator[i].Sub(tileOrigin).Mul(float64(scale))
		dc.SetRGB255(195, 66, 244)
		dc.DrawCircle(p.X, p.Y, 2)
		dc.Fill()
		empty = false
	}

	if empty {
		return emptyPNG, coll.metadata, nil
	}

	var out bytes.Buffer
	dc.EncodePNG(&out)
	return out.Bytes(), coll.metadata, nil
}

func (index *Index) reloadIfChanged(md CollectionMetadata) {
	if coll, err := readCollection(md.Name, md.Path, md.LastModified); err == nil {
		log.Printf("success reading collection %s from %s", md.Name, md.Path)
		index.replaceCollection(coll)
	} else if err == NotModified {
		log.Printf("no change in collection %s at %s",
			md.Name, md.Path)
	} else {
		log.Printf("error reading collection %s at %s: %v",
			md.Name, md.Path, err)
	}
}

func (index *Index) getCollectionMetadata(path string) *CollectionMetadata {
	index.mutex.Lock()
	defer index.mutex.Unlock()

	for _, c := range index.Collections {
		if path == c.metadata.Path {
			return &c.metadata
		}
	}
	return nil
}

func (index *Index) replaceCollection(c *Collection) {
	index.mutex.Lock()
	defer index.mutex.Unlock()
	if old := index.Collections[c.metadata.Name]; old != nil {
		old.Close()
	}
	index.Collections[c.metadata.Name] = c
}

var Modified error = errors.New("FeatureCollection has been modified")
var NotFound error = errors.New("FeatureCollection not found")
var NotModified error = errors.New("FeatureCollection not modified")

// Returns NotModified if the collection has not been modfied since time ifModifiedSince.
func readCollection(name string, path string, ifModifiedSince time.Time) (*Collection, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		numDataLoadErrors.Inc()
		return nil, err
	}

	stat, err := os.Stat(absPath)
	if err != nil {
		numDataLoadErrors.Inc()
		return nil, err
	}

	if !stat.ModTime().After(ifModifiedSince) {
		return nil, NotModified
	}

	data, err := ioutil.ReadFile(absPath)
	if err != nil {
		numDataLoadErrors.Inc()
		return nil, err
	}

	coll := &Collection{}
	coll.metadata.LastModified = stat.ModTime()
	coll.metadata.Name = name
	coll.metadata.Path = absPath

	var features geojson.FeatureCollection
	if err := json.Unmarshal(data, &features); err != nil {
		numDataLoadErrors.Inc()
		return nil, err
	}

	dataFile, err := ioutil.TempFile("", "miniwfs-*.geojson")
	if err != nil {
		return nil, err
	}
	coll.dataFile = dataFile

	headerSize, err := dataFile.Write([]byte(`{"type":"FeatureCollection","features":[\n`))
	if err != nil {
		coll.Close()
		return nil, err
	}
	pos := int64(headerSize)

	numFeatures := len(features.Features)
	coll.bbox = make([]s2.Rect, numFeatures)
	coll.id = make([]string, numFeatures)
	coll.webMercator = make([]r2.Point, numFeatures)
	coll.offset = make([]int64, numFeatures+1)
	coll.byID = make(map[string]int)

	for i, f := range features.Features {
		if id := getIDString(f.ID); len(id) > 0 {
			coll.id[i] = id
			coll.byID[id] = i
		}

		coll.bbox[i] = computeBounds(f.Geometry)
		center := coll.bbox[i].Center()
		coll.webMercator[i] = projectWebMercator(center)

		if i > 0 {
			if _, err := dataFile.Write([]byte(",\n")); err == nil {
				pos += 2
			} else {
				coll.Close()
				return nil, err
			}
		}
		coll.offset[i] = pos

		encoded, err := json.Marshal(f)
		if err != nil {
			coll.Close()
			return nil, err
		}

		if numBytes, err := dataFile.Write(encoded); err == nil {
			pos = pos + int64(numBytes)
		} else {
			coll.Close()
			return nil, err
		}
	}
	coll.offset[len(coll.offset)-1] = pos + 2 // 2 = len(",\n")
	if _, err := dataFile.Write([]byte("\n]}\n")); err != nil {
		coll.Close()
		return nil, err
	}

	// RFC 7946 does not define a "properties" member on FeatureCollection,
	// only on Feature. We still recognize certain collection properties,
	// which is is allowed as per RFC 7946 section 6.1 (Foreign Members).
	type collectionProperties struct {
		Properties map[string]interface{} `json:"properties"`
	}
	var props collectionProperties
	if err := json.Unmarshal(data, &props); err == nil {
		for prop, val := range props.Properties {
			if strings.HasSuffix(prop, "_timestamp") {
				if s, ok := val.(string); ok {
					if t, err := time.Parse(time.RFC3339, s); err == nil {
						propName := strings.TrimSuffix(prop, "_timestamp")
						if len(propName) > 0 {
							collectionTimestamp.WithLabelValues(name, propName).Set(float64(t.UTC().Unix()))

						}
					}
				}
			}
		}
	}

	lastDataLoad.SetToCurrentTime()
	numDataLoads.Inc()
	collectionTimestamp.WithLabelValues(name, "last_modified").Set(float64(coll.metadata.LastModified.UTC().Unix()))
	collectionTimestamp.WithLabelValues(name, "loaded").Set(float64(time.Now().UTC().Unix()))
	collectionFeaturesCount.WithLabelValues(name).Set(float64(numFeatures))

	return coll, nil
}

func getIDString(s interface{}) string {
	if str, ok := s.(string); ok {
		return str
	} else if i, ok := s.(int64); ok {
		return strconv.FormatInt(i, 10)
	} else {
		return ""
	}
}
