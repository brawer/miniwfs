package main

import (
	"reflect"
	"testing"

	"github.com/golang/geo/s2"
)

func TestEncodeBbox(t *testing.T) {
	bbox, _ := parseBbox("8.5,47.9,8.9,49.2")
	got := EncodeBbox(bbox)
	expected := []float64{8.5, 47.9, 8.9, 49.2}
	if !reflect.DeepEqual(expected, got) {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestEncodeBbox_Empty(t *testing.T) {
	got := EncodeBbox(s2.EmptyRect())
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}
