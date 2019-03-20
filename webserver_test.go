package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"github.com/paulmach/go.geojson"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func makeServer(t *testing.T) *WebServer {
	return MakeWebServer(loadTestIndex(t), "https://test.example.org/wfs/")
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

func expectFeatureCollection(t *testing.T, got *geojson.FeatureCollection,
	expected string) {
	gotJSON, err := got.MarshalJSON()
	if err != nil {
		t.Fatalf("cannot marshal to JSON: %s", err)
	}
	expectJSON(t, string(gotJSON), expected)
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

func TestCollections(t *testing.T) {
	s := makeServer(t)
	query, _ := http.NewRequest("GET", "/collections", nil)
	handler := http.HandlerFunc(s.HandleRequest)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, query)

	if ct := resp.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type: application/json, got %s", ct)
	}

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

func TestItem(t *testing.T) {
	s := makeServer(t)
	query, _ := http.NewRequest("GET", "/collections/lakes/items/N123", nil)
	handler := http.HandlerFunc(s.HandleRequest)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, query)

	if ct := resp.Header().Get("Content-Type"); ct != "application/geo+json" {
		t.Errorf("Expected Content-Type: application/geo+json, got %s", ct)
	}

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
