package main

import (
	"github.com/paulmach/go.geojson"
)

type WFSLink struct {
	Href  string `json:"href"`
	Rel   string `json:"rel"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

type WFSFeatureCollection struct {
	Type        string             `json:"type"`
	Links       []*WFSLink         `json:"links,omitempty"`
	BoundingBox []float64          `json:"bbox,omitempty"`
	Features    []*geojson.Feature `json:"features"`
}
