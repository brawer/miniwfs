package main

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/golang/geo/s2"
	"github.com/paulmach/go.geojson"
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

func TestGetCollections(t *testing.T) {
	got := loadTestIndex(t).GetCollections()
	expected := []string{"castles", "lakes"}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

func TestGetItem_ExistingItem(t *testing.T) {
	got := loadTestIndex(t).GetItem("castles", "W418392510")
	if got == nil || got.Properties["name"] != "Castello Scaligero" {
		t.Fatalf("expected W418392510, got %v", got)
	}
}

func TestGetItem_NoSuchCollection(t *testing.T) {
	got := loadTestIndex(t).GetItem("no-such-collection", "123")
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestGetItem_NoSuchItem(t *testing.T) {
	got := loadTestIndex(t).GetItem("castles", "unknown-id")
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestGetItems_EmptyBbox(t *testing.T) {
	got := loadTestIndex(t).GetItems("castles", "", 0, DefaultLimit, s2.EmptyRect())
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
	// startID=W418392510 is a known ID (of feature #1)
	// start=77 should be ignored when startID is a known ID
	got := loadTestIndex(t).GetItems("castles", "W418392510", 77, 2, s2.FullRect())
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
	got := loadTestIndex(t).GetItems("castles", "UnknownID", 2, 2, s2.FullRect())
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
	got := loadTestIndex(t).GetItems("castles", "", 2, 2, s2.FullRect())
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
	got := loadTestIndex(t).GetItems("castles", "", 0, 2, s2.FullRect())
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
	got := loadTestIndex(t).GetItems("no-such-collection", "", 0, DefaultLimit, s2.FullRect())
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func getFeatureIDs(f []*geojson.Feature) string {
	ids := make([]string, len(f))
	for i, feat := range f {
		ids[i] = getIDString(feat.ID)
	}
	return strings.Join(ids, ",")
}
