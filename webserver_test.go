package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang/geo/s2"
)

func makeServer(t *testing.T) *WebServer {
	return MakeWebServer(loadTestIndex(t))
}

func getBody(r *httptest.ResponseRecorder) string {
	var stream io.Reader = r.Body
	var err error
	if r.Header().Get("Content-Encoding") == "gzip" {
		stream, err = gzip.NewReader(r.Body)
		if err != nil {
			return err.Error()
		}
	}
	b, err := ioutil.ReadAll(stream)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func expectJSON(t *testing.T, got string, expected string) {
	var prettyGot bytes.Buffer
	if err := json.Indent(&prettyGot, []byte(got), "", "  "); err != nil {
		t.Fatalf("error pretty-printing JSON: %s", err)
	}

	var prettyExpected bytes.Buffer
	if err := json.Indent(&prettyExpected, []byte(expected), "", "  "); err != nil {
		t.Fatalf("error pretty-printing JSON: %s", err)
	}

	if prettyGot.String() != prettyExpected.String() {
		t.Fatalf("expected: %s\ngot:      %s\n",
			prettyExpected.String(), prettyGot.String())
	}
}

func expectFeatureCollection(t *testing.T, got *WFSFeatureCollection,
	expected string) {
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("cannot marshal to JSON: %s", err)
	}
	expectJSON(t, string(gotJSON), expected)
}

func TestParseBbox_emptyString(t *testing.T) {
	bbox, err := parseBbox("")
	if !bbox.IsFull() || err != nil {
		t.Errorf("expected (full-bbox, nil), got (%s, %s)", bbox, err)
	}
}

func TestParseBbox_2D(t *testing.T) {
	bbox, err := parseBbox(" -8.5, -47.9, -8.9 , -49.2")
	if err != nil {
		t.Errorf("expected nil error, got %s", err)
		return
	}

	if bbox.Lo().Distance(s2.LatLngFromDegrees(-49.2, -8.9)) > 0.001 {
		t.Errorf("expected bbox.Lo=(-49.2, -8.9) error, got %s", bbox.Lo())
	}

	if bbox.Hi().Distance(s2.LatLngFromDegrees(-47.9, -8.5)) > 0.001 {
		t.Errorf("expected bbox.Hi=(-47.9, -8.5) error, got %s", bbox.Lo())
	}
}

func TestParseBbox_3D(t *testing.T) {
	bbox, err := parseBbox("-8.5,-47.9,-100,-8.9,-49.2,1400")
	if err != nil {
		t.Errorf("expected nil error, got %s", err)
		return
	}

	if bbox.Lo().Distance(s2.LatLngFromDegrees(-49.2, -8.9)) > 0.001 {
		t.Errorf("expected bbox.Lo=(-49.2, -8.9) error, got %s", bbox.Lo())
	}

	if bbox.Hi().Distance(s2.LatLngFromDegrees(-47.9, -8.5)) > 0.001 {
		t.Errorf("expected bbox.Hi=(-47.9, -8.5) error, got %s", bbox.Lo())
	}
}

func TestHome(t *testing.T) {
	s := makeServer(t)
	query, _ := http.NewRequest("GET", "/", nil)
	handler := http.HandlerFunc(s.HandleRequest)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, query)

	if ct := resp.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Expected Content-Type HTML, got %s", ct)
	}

	body := getBody(resp)
	if !strings.Contains(body, "WFS3") {
		t.Errorf("Expected homepage; got %s", body)
	}
}

func TestListCollections(t *testing.T) {
	s := makeServer(t)
	query, _ := http.NewRequest("GET", "/collections", nil)
	handler := http.HandlerFunc(s.HandleRequest)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, query)

	if ct := resp.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type: application/json, got %s", ct)
	}

	expectCORSHeader(t, resp.Header())
	expectJSON(t, getBody(resp), `{
          "links": [
            {
              "href": "https://test.example.org/wfs/collections",
              "rel": "self",
              "type": "application/json",
              "title": "Collections"
            }
          ],
          "collections": [
            {
              "name": "castles",
              "links": [
                {
                  "href": "https://test.example.org/wfs/collections/castles",
                  "rel": "item",
                  "type": "application/geo+json",
                  "title": "castles"
                }
              ]
            },
            {
              "name": "lakes",
              "links": [
                {
                  "href": "https://test.example.org/wfs/collections/lakes",
                  "rel": "item",
                  "type": "application/geo+json",
                  "title": "lakes"
                }
              ]
            }
          ]
        }`)
}

func TestCollection(t *testing.T) {
	s := makeServer(t)
	query, _ := http.NewRequest("GET", "/collections/castles/items?bbox=11.183467,47.910413,11.183469,47.910415", nil)
	handler := http.HandlerFunc(s.HandleRequest)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, query)

	if ct := resp.Header().Get("Content-Type"); ct != "application/geo+json" {
		t.Errorf("Expected Content-Type: application/geo+json, got %s", ct)
	}

	stat, _ := os.Stat(filepath.Join("testdata", "castles.geojson"))
	expectedLastModified := stat.ModTime().UTC().Format(http.TimeFormat)
	gotLastModified := resp.Header().Get("Last-Modified")
	if expectedLastModified != gotLastModified {
		t.Errorf("Expected Last-Modified: %s, got %s", expectedLastModified, gotLastModified)
	}

	expectCORSHeader(t, resp.Header())
	expectJSON(t, getBody(resp), `{
          "type": "FeatureCollection",
          "links": [
            {
              "href": "https://test.example.org/wfs/collections/castles/items?bbox=11.1834670,47.9104130,11.1834690,47.9104150",
              "rel": "self",
              "type": "application/geo+json",
              "title": "self"
            }
          ],
          "bbox": [
            11.183468,
            47.910414,
            11.183468,
            47.910414
          ],
          "features": [
            {
              "id": "N34729562",
              "type": "Feature",
              "geometry": {
                "type": "Point",
                "coordinates": [
                  11.183468,
                  47.910414
                ]
              },
              "properties": {
                "historic": "castle",
                "name": "Hochschloß Pähl"
              }
            }
          ]
        }`)
}

func TestItem(t *testing.T) {
	s := makeServer(t)
	query, _ := http.NewRequest("GET", "/collections/lakes/items/N123", nil)
	handler := http.HandlerFunc(s.HandleRequest)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, query)

	if ct := resp.Header().Get("Content-Type"); ct != "application/geo+json" {
		t.Errorf("Expected Content-Type: application/geo+json, got %s", ct)
	}

	expectCORSHeader(t, resp.Header())
	expectJSON(t, getBody(resp), `{
          "id": "N123",
          "type": "Feature",
          "geometry": {
            "type": "Point",
            "coordinates": [
              11.183468,
              47.910414
            ]
          },
          "properties": {
            "name": "Katzensee",
            "natural": "lake"
          }
        }`)
}

func expectCORSHeader(t *testing.T, header http.Header) {
	if cors := header.Get("Access-Control-Allow-Origin"); cors != "*" {
		t.Errorf("expected header \"Access-Control-Allow-Origin: *\", got %s", cors)
	}
}
