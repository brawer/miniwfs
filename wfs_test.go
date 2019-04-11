package main

import (
	"github.com/golang/geo/s2"
	"testing"
)

func TestFormatItemsURL(t *testing.T) {
	bbox, _ := parseBbox("8.5,47.9,8.9,49.2")
	got := FormatItemsURL("http://foo.org/bar/", "lakés", "ä123", 123, 99, bbox)
	expected := "http://foo.org/bar/collections/lak%C3%A9s/items?startID=%C3%A4123&start=123&limit=99&bbox=8.5000000,47.9000000,8.9000000,49.2000000"
	if expected != got {
		t.Errorf("expected \"%s\", got \"%s\"", expected, got)
	}
}

func TestFormatItemsURL_DefaultParams(t *testing.T) {
	got := FormatItemsURL("http://foo.org/bar/", "lakes", "", 0, DefaultLimit, s2.FullRect())
	expected := "http://foo.org/bar/collections/lakes/items"
	if expected != got {
		t.Errorf("expected \"%s\", got \"%s\"", expected, got)
	}
}

func TestFormatItemsURL_EmptyBbox(t *testing.T) {
	got := FormatItemsURL("http://foo.org/bar/", "lakes", "", 0, DefaultLimit, s2.EmptyRect())
	expected := "http://foo.org/bar/collections/lakes/items"
	if expected != got {
		t.Errorf("expected \"%s\", got \"%s\"", expected, got)
	}
}
