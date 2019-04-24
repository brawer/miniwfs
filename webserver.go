package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	//"fmt"
	"html"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/golang/geo/s1"
	"github.com/golang/geo/s2"
)

type WebServer struct {
	index                *Index
	httpServer           http.Server
	shutdownHasCompleted chan struct{}
}

func MakeWebServer(index *Index) *WebServer {
	s := &WebServer{index: index, shutdownHasCompleted: make(chan struct{})}
	return s
}

var collectionRegexp = regexp.MustCompile(`^/collections/([^/]+)/items$`)
var itemRegexp = regexp.MustCompile(`^/collections/([^/]+)/items/(.+)$`)
var listCollectionsRegexp = regexp.MustCompile(`^/collections/?$`)
var tilesRegexp = regexp.MustCompile(
	`^/tiles/([^/]+)/([^/]+)/([^/]+)/([^/]+)\.png$`)
var tileFeatureInfoRegexp = regexp.MustCompile(
	`^/tiles/([^/]+)/([0-9]+)/([0-9]+)/([0-9]+)/([0-9]+)/([0-9]+)\.geojson$`)

func (s *WebServer) ListenAndServe(port int) error {
	s.httpServer.Addr = ":" + strconv.Itoa(port)
	err := s.httpServer.ListenAndServe()
	<-s.shutdownHasCompleted
	return err
}

func (s *WebServer) Shutdown() {
	s.httpServer.Shutdown(context.Background())
	close(s.shutdownHasCompleted)
}

func (s *WebServer) HandleRequest(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if m := tilesRegexp.FindStringSubmatch(path); len(m) == 5 {
		zoom, _ := strconv.Atoi(m[2])
		x, _ := strconv.Atoi(m[3])
		y, _ := strconv.Atoi(m[4])
		s.handleTileRequest(w, req, m[1], zoom, x, y)
		return
	}

	if m := tileFeatureInfoRegexp.FindStringSubmatch(path); len(m) == 7 {
		collection := m[1]
		zoom, _ := strconv.ParseUint(m[2], 10, 8)
		x, _ := strconv.ParseUint(m[3], 10, 32)
		y, _ := strconv.ParseUint(m[4], 10, 32)
		i, _ := strconv.ParseUint(m[5], 10, 32)
		j, _ := strconv.ParseUint(m[6], 10, 32)
		tile := &TileKey{X: uint32(x), Y: uint32(y), Zoom: uint8(zoom)}
		s.handleTileFeatureInfoRequest(w, req, collection, tile, int(i), int(j))
		return
	}

	if m := collectionRegexp.FindStringSubmatch(path); len(m) == 2 {
		s.handleCollectionRequest(w, req, m[1])
		return
	}

	if m := itemRegexp.FindStringSubmatch(path); len(m) == 3 {
		s.handleItemRequest(w, req, m[1], m[2])
		return
	}

	if m := listCollectionsRegexp.FindStringSubmatch(path); len(m) == 1 {
		s.handleListCollectionsRequest(w, req)
		return
	}

	if req.URL.Path == "/" {
		s.handleHomeRequest(w, req)
	}

	w.WriteHeader(http.StatusNotFound)
}

func (s *WebServer) handleHomeRequest(w http.ResponseWriter, req *http.Request) {
	url := html.EscapeString(s.index.PublicPath.String() + "collections")

	var out bytes.Buffer
	out.WriteString(
		"<html><body><h1>MiniWFS</h1>" +
			"<p>Hello! This is a <a href=\"https://github.com/brawer/miniwfs\">" +
			"MiniWFS</a> server. To use it, point any WFS3 client to <a href=\"")
	out.WriteString(url)
	out.WriteString("\">")
	out.WriteString(url)
	out.WriteString("</a>.</p></html>")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	out.WriteTo(w)

}

func (s *WebServer) handleListCollectionsRequest(w http.ResponseWriter, req *http.Request) {
	type WFSCollection struct {
		Name  string    `json:"name"`
		Links []WFSLink `json:"links"`
	}

	type WFSCollectionResponse struct {
		Links       []WFSLink       `json:"links"`
		Collections []WFSCollection `json:"collections"`
	}

	collections := s.index.GetCollections()
	wfsCollections := make([]WFSCollection, 0, len(collections))
	for _, c := range collections {
		link := WFSLink{
			Href:  s.index.PublicPath.String() + "collections/" + c.Name,
			Rel:   "item",
			Type:  "application/geo+json",
			Title: c.Name,
		}
		wfsColl := WFSCollection{Name: c.Name, Links: []WFSLink{link}}
		wfsCollections = append(wfsCollections, wfsColl)
	}

	selfLink := WFSLink{
		Href: s.index.PublicPath.String() + "collections",
		Rel:  "self", Type: "application/json", Title: "Collections",
	}

	result := WFSCollectionResponse{
		Links:       []WFSLink{selfLink},
		Collections: wfsCollections,
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		log.Printf("json.Marshal failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(encoded)
}

func (s *WebServer) handleCollectionRequest(w http.ResponseWriter, req *http.Request,
	collection string) {
	params := req.URL.Query()

	ifModifiedSince, _ := http.ParseTime(req.Header.Get("If-Modified-Since"))
	ifUnmodifiedSince, _ := http.ParseTime(req.Header.Get("If-Unmodified-Since"))

	start := 0
	startParam := strings.TrimSpace(params.Get("start"))
	if len(startParam) > 0 {
		var err error
		start, err = strconv.Atoi(startParam)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	startID := params.Get("startID")

	limit := DefaultLimit
	limitParam := strings.TrimSpace(params.Get("limit"))
	if len(limitParam) > 0 {
		var err error
		limit, err = strconv.Atoi(limitParam)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	bbox, err := parseBbox(params.Get("bbox"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var buf bytes.Buffer
	includeLinks := true
	metadata, err := s.index.GetItems(collection, startID, start, limit, bbox,
		ifModifiedSince, ifUnmodifiedSince, includeLinks, &buf)
	if status := getHTTPStatus(err); status != http.StatusOK {
		w.WriteHeader(status)
		return
	}

	header := w.Header()
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Content-Length", strconv.Itoa(buf.Len()))
	header.Set("Content-Type", "application/geo+json")
	header.Set("Last-Modified", metadata.LastModified.UTC().Format(http.TimeFormat))

	w.WriteHeader(http.StatusOK)
	buf.WriteTo(w)
}

var malformedBbox error = errors.New("malformed bbox parameter")

func parseBbox(s string) (s2.Rect, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return s2.FullRect(), nil
	}

	bbox := s2.EmptyRect()
	parts := strings.Split(s, ",")
	n := make([]float64, len(parts))
	var err error
	for i, part := range parts {
		n[i], err = strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return bbox, err
		}
	}

	if len(n) == 4 {
		bbox = bbox.AddPoint(s2.LatLngFromDegrees(n[1], n[0]))
		bbox = bbox.AddPoint(s2.LatLngFromDegrees(n[3], n[2]))
		if bbox.IsValid() {
			return bbox, nil
		}
	}

	if len(n) == 6 {
		bbox = bbox.AddPoint(s2.LatLngFromDegrees(n[1], n[0]))
		bbox = bbox.AddPoint(s2.LatLngFromDegrees(n[4], n[3]))
		if bbox.IsValid() {
			return bbox, nil
		}
	}

	return s2.EmptyRect(), malformedBbox
}

func (s *WebServer) handleItemRequest(w http.ResponseWriter, req *http.Request,
	collection string, item string) {
	feature, err := s.index.GetItem(collection, item)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if feature == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	encoded, err := json.Marshal(feature)
	if err != nil {
		log.Printf("json.Marshal failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/geo+json")
	w.WriteHeader(http.StatusOK)
	w.Write(encoded)
}

func (s *WebServer) handleTileRequest(w http.ResponseWriter, req *http.Request,
	collection string, zoom int, x int, y int) {
	tile, metadata, err := s.index.GetTile(collection, zoom, x, y)
	if status := getHTTPStatus(err); status != http.StatusOK {
		w.WriteHeader(status)
		return
	}

	header := w.Header()
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Content-Length", strconv.Itoa(len(tile)))
	header.Set("Content-Type", "image/png")
	header.Set("Last-Modified", metadata.LastModified.UTC().Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)
	w.Write(tile)
}

func (s *WebServer) handleTileFeatureInfoRequest(
	w http.ResponseWriter, req *http.Request,
	collection string, tile *TileKey, i int, j int) {
	if i < 0 || i > 256 || j < 0 || j >= 256 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tileBounds := tile.Bounds()
	tileSize := tileBounds.Size()
	pixelSize := s2.LatLng{Lat: tileSize.Lat / 256, Lng: tileSize.Lng / 256}
	center := s2.LatLng{
		Lat: s1.Angle(tileBounds.Hi().Lat.Radians() - pixelSize.Lat.Radians()*float64(j)),
		Lng: s1.Angle(tileBounds.Lo().Lng.Radians() + pixelSize.Lng.Radians()*float64(i))}
	maxSignatureWidth := 8.0 // pixels
	bboxSize := s2.LatLng{
		Lat: s1.Angle(pixelSize.Lat.Radians() * maxSignatureWidth),
		Lng: s1.Angle(pixelSize.Lng.Radians() * maxSignatureWidth)}
	bbox := s2.RectFromCenterSize(center, bboxSize)

	ifModifiedSince, _ := http.ParseTime(req.Header.Get("If-Modified-Since"))
	ifUnmodifiedSince, _ := http.ParseTime(req.Header.Get("If-Unmodified-Since"))
	limit := 10
	includeLinks := false
	var buf bytes.Buffer
	metadata, err := s.index.GetItems(collection, "", 0, limit, bbox,
		ifModifiedSince, ifUnmodifiedSince, includeLinks, &buf)
	if status := getHTTPStatus(err); status != http.StatusOK {
		w.WriteHeader(status)
		return
	}

	header := w.Header()
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Content-Length", strconv.Itoa(buf.Len()))
	header.Set("Content-Type", "application/geo+json")
	header.Set("Last-Modified", metadata.LastModified.UTC().Format(http.TimeFormat))

	w.WriteHeader(http.StatusOK)
	buf.WriteTo(w)
}

func getHTTPStatus(err error) int {
	switch err {
	case nil:
		return http.StatusOK

	case Modified:
		return http.StatusPreconditionFailed

	case NotFound:
		return http.StatusNotFound

	case NotModified:
		return http.StatusNotModified

	default:
		return http.StatusInternalServerError
	}
}
