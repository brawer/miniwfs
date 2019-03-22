package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/golang/geo/s2"
	"github.com/paulmach/go.geojson"
)

const DefaultLimit = 10
const MaxLimit = 10000

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

func EncodeBbox(r s2.Rect) []float64 {
	if r.IsEmpty() {
		return nil
	} else {
		bbox := [4]float64{
			r.Lo().Lng.Degrees(),
			r.Lo().Lat.Degrees(),
			r.Hi().Lng.Degrees(),
			r.Hi().Lat.Degrees(),
		}
		return bbox[0:4]
	}
}

func FormatItemsURL(prefix string, collection string,
	startID string, start int, limit int, bbox s2.Rect) string {
	params := make([]string, 0, 4)
	if len(startID) > 0 {
		params = append(params, "startID="+url.QueryEscape(startID))
	}
	if start > 0 {
		params = append(params, fmt.Sprintf("start=%d", start))
	}
	if limit != DefaultLimit {
		params = append(params, fmt.Sprintf("limit=%d", limit))
	}
	if !bbox.IsFull() {
		r := EncodeBbox(bbox)
		if r != nil {
				boxParam := fmt.Sprintf("bbox=%.7f,%.7f,%.7f,%.7f", r[0], r[1], r[2], r[3])
				params = append(params, boxParam)
		}
	}
	u := prefix + "collections/" + url.PathEscape(collection) + "/items"
	if len(params) > 0 {
		return u + "?" + strings.Join(params, "&")
	} else {
		return u
	}
}
