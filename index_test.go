package main

import (
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
