package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/golang/geo/s2"
	"github.com/paulmach/go.geojson"
	promtest "github.com/prometheus/client_golang/prometheus/testutil"
)

func loadTestIndex(t *testing.T) *Index {
	p1 := filepath.Join("testdata", "castles.geojson")
	p2 := filepath.Join("testdata", "lakes.geojson")

	publicPath, _ := url.Parse("https://test.example.org/wfs/")
	index, err := MakeIndex(map[string]string{"castles": p1, "lakes": p2},
		publicPath)
	if index == nil || err != nil {
		t.Fatalf("failed making index: %s", err)
	}
	return index
}

var noTime time.Time

func TestGetCollections(t *testing.T) {
	index := loadTestIndex(t)
	defer index.Close()
	colls := index.GetCollections()
	got := make([]string, len(colls))
	for i, c := range colls {
		got[i] = c.Name
	}
	expected := []string{"castles", "lakes"}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

func TestGetItem_ExistingItem(t *testing.T) {
	index := loadTestIndex(t)
	defer index.Close()
	got, _ := index.GetItem("castles", "W418392510")
	if got == nil || got.Properties["name"] != "Castello Scaligero" {
		t.Fatalf("expected W418392510, got %v", got)
	}
}

func TestGetItem_NoSuchCollection(t *testing.T) {
	index := loadTestIndex(t)
	defer index.Close()
	got, _ := index.GetItem("no-such-collection", "123")
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestGetItem_NoSuchItem(t *testing.T) {
	index := loadTestIndex(t)
	defer index.Close()
	got, _ := index.GetItem("castles", "unknown-id")
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func getItems(index *Index, collection string, startID string, startIndex int, limit int, bbox s2.Rect) (*WFSFeatureCollection, *CollectionMetadata, error) {
	var buf bytes.Buffer
	md, err := index.GetItems(collection, startID, startIndex, limit, bbox,
		noTime, noTime, &buf)
	if err != nil {
		return nil, nil, err
	}

	var result WFSFeatureCollection
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		return nil, nil, err
	}

	return &result, &md, nil
}

func TestGetItems_EmptyBbox(t *testing.T) {
	index := loadTestIndex(t)
	defer index.Close()

	got, _, _ := getItems(index, "castles", "", 0, DefaultLimit, s2.EmptyRect())
	expectFeatureCollection(t, got, `{
		"type": "FeatureCollection",
		"links": [{
			"href": "https://test.example.org/wfs/collections/castles/items",
			"rel": "self",
			"type": "application/geo+json",
			"title": "self"
		}],
		"features": []
	}`)
}

func TestGetItems_KnownStartID(t *testing.T) {
	index := loadTestIndex(t)
	defer index.Close()
	// startID=W418392510 is a known ID (of feature #1)
	// start=77 should be ignored when startID is a known ID
	got, _, _ := getItems(index, "castles", "W418392510", 77, 2, s2.FullRect())
	gotIDs := getFeatureIDs(got.Features)
	expectedIDs := "W418392510,W24785843"
	if expectedIDs != gotIDs {
		t.Errorf("expected %s, got %s", expectedIDs, gotIDs)
		return
	}
	links, _ := json.Marshal(got.Links)
	expectJSON(t, string(links), `[
          {
            "href": "https://test.example.org/wfs/collections/castles/items?startID=W418392510\u0026start=1\u0026limit=2",
            "rel": "self",
            "type": "application/geo+json",
            "title": "self"
          }
        ]`)
}

func TestGetItems_UnknownStartID(t *testing.T) {
	index := loadTestIndex(t)
	defer index.Close()
	got, _, _ := getItems(index, "castles", "UnknownID", 2, 2, s2.FullRect())
	gotIDs := getFeatureIDs(got.Features)
	expectedIDs := "W24785843"
	if expectedIDs != gotIDs {
		t.Errorf("expected %s, got %s", expectedIDs, gotIDs)
		return
	}
	links, _ := json.Marshal(got.Links)
	expectJSON(t, string(links), `[
          {
            "href": "https://test.example.org/wfs/collections/castles/items?startID=UnknownID\u0026start=2\u0026limit=2",
            "rel": "self",
            "type": "application/geo+json",
            "title": "self"
          }
        ]`)
}

func TestGetItems_NoStartID(t *testing.T) {
	index := loadTestIndex(t)
	defer index.Close()
	got, _, _ := getItems(index, "castles", "", 2, 2, s2.FullRect())
	gotIDs := getFeatureIDs(got.Features)
	expectedIDs := "W24785843"
	if expectedIDs != gotIDs {
		t.Errorf("expected %s, got %s", expectedIDs, gotIDs)
		return
	}
	links, _ := json.Marshal(got.Links)
	expectJSON(t, string(links), `[
          {
            "href": "https://test.example.org/wfs/collections/castles/items?start=2\u0026limit=2",
            "rel": "self",
            "type": "application/geo+json",
            "title": "self"
          }
        ]`)
}

func TestGetItems_LimitExceeded(t *testing.T) {
	index := loadTestIndex(t)
	defer index.Close()
	got, _, _ := getItems(index, "castles", "", 0, 2, s2.FullRect())
	gotIDs := getFeatureIDs(got.Features)
	expectedIDs := "N34729562,W418392510"
	if expectedIDs != gotIDs {
		t.Errorf("expected %s, got %s", expectedIDs, gotIDs)
		return
	}
	links, _ := json.Marshal(got.Links)
	expectJSON(t, string(links), `[
          {
            "href": "https://test.example.org/wfs/collections/castles/items?limit=2",
            "rel": "self",
            "type": "application/geo+json",
            "title": "self"
          },
          {
            "href": "https://test.example.org/wfs/collections/castles/items?startID=W24785843\u0026start=2\u0026limit=2",
            "rel": "next",
            "type": "application/geo+json",
            "title": "next"
          }
        ]`)
}

func TestGetItems_NoSuchCollection(t *testing.T) {
	index := loadTestIndex(t)
	defer index.Close()
	_, _, err := getItems(index, "no-such-collection", "", 0, DefaultLimit, s2.FullRect())
	if err != NotFound {
		t.Errorf("expected %v, got %v", NotFound, err)
	}
}

func TestGetItems_Metadata(t *testing.T) {
	index := loadTestIndex(t)
	defer index.Close()
	stat, _ := os.Stat(filepath.Join("testdata", "castles.geojson"))
	expectedLastModified := stat.ModTime().UTC().Format(time.RFC3339)
	_, md, _ := getItems(index, "castles", "", 0, 2, s2.FullRect())
	gotLastModified := md.LastModified.UTC().Format(time.RFC3339)
	if expectedLastModified != gotLastModified {
		t.Errorf("exptected LastModified=%s in metadata, got %s",
			expectedLastModified, gotLastModified)
	}
}

func TestReadCollection_CollectionMetrics(t *testing.T) {
	index := loadTestIndex(t)
	defer index.Close()
	m, _ := collectionFeaturesCount.GetMetricWithLabelValues("castles")
	if promtest.ToFloat64(m) != 3 {
		t.Fatalf("expected collectionFeaturesCount == 3 for castles, got %v", promtest.ToFloat64(m))
	}

	expected := "2019-04-04T16:09:03Z"
	m, _ = collectionTimestamp.GetMetricWithLabelValues("castles", "osm_base")
	osmBase := time.Unix(int64(promtest.ToFloat64(m)), 0).UTC().Format(time.RFC3339)
	if osmBase != expected {
		t.Fatalf("expected timestamp %s for castles/osm_base, got %s", expected, osmBase)
	}

	m, _ = collectionTimestamp.GetMetricWithLabelValues("castles", "last_modified")
	lastMod := time.Unix(int64(promtest.ToFloat64(m)), 0).UTC().Format(time.RFC3339)
	stat, _ := os.Stat(filepath.Join("testdata", "castles.geojson"))
	expected = stat.ModTime().UTC().Format(time.RFC3339)
	if lastMod != expected {
		t.Errorf("expected timestamp %s for castles/last_modified, got %s", expected, lastMod)
	}

	m, _ = collectionTimestamp.GetMetricWithLabelValues("castles", "loaded")
	loaded := time.Unix(int64(promtest.ToFloat64(m)), 0).UTC()
	delta := time.Since(loaded)
	if delta.Seconds() > 10.0 {
		t.Fatalf("expected timestamp for castles/loaded within 10s from now, got %s", delta)
	}
}

func TestReadCollection_IfModifiedSince(t *testing.T) {
	tmpfile, _ := ioutil.TempFile("", "test.*.geojson")
	defer os.Remove(tmpfile.Name())
	tmpfile.Write([]byte(`{"features":[]}`))
	tmpfile.Close()

	t1 := time.Date(2001, time.February, 1, 3, 4, 5, 0, time.UTC)
	t2 := time.Date(2002, time.February, 1, 3, 4, 5, 0, time.UTC)
	t3 := time.Date(2003, time.February, 1, 3, 4, 5, 0, time.UTC)

	os.Chtimes(tmpfile.Name(), t1, t1)
	if _, err := readCollection("test", tmpfile.Name(), t1); err != NotModified {
		t.Errorf("expected NotModified for mod=T1/ifModifiedSince=T1, got %v", err)
	}
	if _, err := readCollection("test", tmpfile.Name(), t2); err != NotModified {
		t.Errorf("expected NotModified for mod=T1/ifModifiedSince=T2, got %v", err)
	}
	if _, err := readCollection("test", tmpfile.Name(), t3); err != NotModified {
		t.Errorf("expected NotModified for mod=T1/ifModifiedSince=T3, got %v", err)
	}

	os.Chtimes(tmpfile.Name(), t2, t2)
	if _, err := readCollection("test", tmpfile.Name(), t1); err != nil {
		t.Errorf("expected no error for mod=T2/ifModifiedSince=T1, got %v", err)
	}
	if _, err := readCollection("test", tmpfile.Name(), t2); err != NotModified {
		t.Errorf("expected NotModified for mod=T2/ifModifiedSince=T2, got %v", err)
	}
	if _, err := readCollection("test", tmpfile.Name(), t3); err != NotModified {
		t.Errorf("expected NotModified for mod=T2/ifModifiedSince=T3, got %v", err)
	}

	os.Chtimes(tmpfile.Name(), t3, t3)
	if _, err := readCollection("test", tmpfile.Name(), t1); err != nil {
		t.Errorf("expected no error for mod=T3/ifModifiedSince=T1, got %v", err)
	}
	if _, err := readCollection("test", tmpfile.Name(), t2); err != nil {
		t.Errorf("expected no error for mod=T3/ifModifiedSince=T2, got %v", err)
	}
	if _, err := readCollection("test", tmpfile.Name(), t3); err != NotModified {
		t.Errorf("expected NotModified for mod=T3/ifModifiedSince=T3, got %v", err)
	}
}

func getFeatureIDs(f []*geojson.Feature) string {
	ids := make([]string, len(f))
	for i, feat := range f {
		ids[i] = getIDString(feat.ID)
	}
	return strings.Join(ids, ",")
}
