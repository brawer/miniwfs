package main

import (
	"encoding/json"
	//"fmt"
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

var itemRegexp = regexp.MustCompile(`^/collections/([^/]+)/items/(.+)$`)

func (s *WebServer) HandleCollections(w http.ResponseWriter, req *http.Request) {
	if m := itemRegexp.FindStringSubmatch(req.URL.Path); len(m) == 3 {
		s.HandleItemRequest(w, req, m[1], m[2])
		return
	}

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

func (s *WebServer) HandleItemRequest(w http.ResponseWriter, req *http.Request,
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
