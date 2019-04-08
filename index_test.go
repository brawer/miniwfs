package main

import (
	"encoding/json"
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

func TestReadCollection_CollectionMetrics(t *testing.T) {
	loadTestIndex(t)

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

func getFeatureIDs(f []*geojson.Feature) string {
	ids := make([]string, len(f))
	for i, feat := range f {
		ids[i] = getIDString(feat.ID)
	}
	return strings.Join(ids, ",")
}
