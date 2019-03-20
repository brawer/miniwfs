package main

import (
	"bytes"
	"encoding/json"
	//"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"regexp"
)

type WebServer struct {
	index      *Index
	publicPath *url.URL
}

func MakeWebServer(index *Index, publicPathPrefix string) *WebServer {
	publicPath, err := url.Parse(publicPathPrefix)
	if err != nil {
		log.Fatal(err)
	}
	s := &WebServer{index: index, publicPath: publicPath}
	return s
}

var collectionsRegexp = regexp.MustCompile(`^/collections/?$`)
var itemRegexp = regexp.MustCompile(`^/collections/([^/]+)/items/(.+)$`)

func (s *WebServer) HandleRequest(w http.ResponseWriter, req *http.Request) {
	if m := itemRegexp.FindStringSubmatch(req.URL.Path); len(m) == 3 {
		s.handleItemRequest(w, req, m[1], m[2])
		return
	}

	if m := collectionsRegexp.FindStringSubmatch(req.URL.Path); len(m) == 1 {
		s.handleCollectionsRequest(w, req)
		return
	}

	if req.URL.Path == "/" {
		s.handleHomeRequest(w, req)
	}
	w.WriteHeader(http.StatusNotFound)
}

func (s *WebServer) handleHomeRequest(w http.ResponseWriter, req *http.Request) {
	url := html.EscapeString(s.publicPath.String() + "collections")

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

func (s *WebServer) handleCollectionsRequest(w http.ResponseWriter, req *http.Request) {
	type WFSLink struct {
		Href  string `json:"href"`
		Rel   string `json:"rel"`
		Type  string `json:"type"`
		Title string `json:"title"`
	}
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
	for _, name := range collections {
		link := WFSLink{
			Href:  s.publicPath.String() + "collections/" + name,
			Rel:   "item",
			Type:  "application/geo+json",
			Title: name,
		}
		wfsColl := WFSCollection{Name: name, Links: []WFSLink{link}}
		wfsCollections = append(wfsCollections, wfsColl)
	}

	selfLink := WFSLink{
		Href: s.publicPath.String() + "collections",
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(encoded)
}

func (s *WebServer) handleItemRequest(w http.ResponseWriter, req *http.Request,
	collection string, item string) {
	feature := s.index.GetItem(collection, item)
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

	w.Header().Set("Content-Type", "application/geo+json")
	w.WriteHeader(http.StatusOK)
	w.Write(encoded)
}
