package main

import (
	"github.com/golang/geo/s2"
	"path/filepath"
	"reflect"
	"testing"
)

func loadTestIndex(t *testing.T) *Index {
	p1 := filepath.Join("testdata", "castles.geojson")
	p2 := filepath.Join("testdata", "lakes.geojson")
	index, err := MakeIndex(map[string]string{"castles": p1, "lakes": p2})
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
	got := loadTestIndex(t).GetItems("castles", s2.EmptyRect())
	expectFeatureCollection(t, got, `{
		"type": "FeatureCollection",
		"features": []
	}`)
}

func TestGetItems_NoSuchCollection(t *testing.T) {
	got := loadTestIndex(t).GetItems("no-such-collection", s2.FullRect())
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}
