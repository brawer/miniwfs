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
	"time"

	"github.com/golang/geo/s2"
)

func makeServer(t *testing.T) (*Index, *WebServer) {
	index := loadTestIndex(t)
	return index, MakeWebServer(index)
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
	index, s := makeServer(t)
	defer s.Shutdown()
	defer index.Close()

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
	index, s := makeServer(t)
	defer s.Shutdown()
	defer index.Close()
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
	index, s := makeServer(t)
	defer s.Shutdown()
	defer index.Close()
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
          ],
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
          ]
        }`)
}

func TestCollection_NotFound(t *testing.T) {
	index, s := makeServer(t)
	defer s.Shutdown()
	defer index.Close()
	query, _ := http.NewRequest("GET", "/collections/nosuchcollection/items", nil)
	handler := http.HandlerFunc(s.HandleRequest)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, query)
	r := resp.Result()
	if r.StatusCode != http.StatusNotFound {
		t.Errorf("expected %d, got %d", http.StatusNotFound, r.StatusCode)
	}
}

func TestCollection_IfModifiedSince(t *testing.T) {
	stat, _ := os.Stat(filepath.Join("testdata", "castles.geojson"))
	past := stat.ModTime().Add(-time.Hour).UTC().Format(http.TimeFormat)
	present := stat.ModTime().UTC().Format(http.TimeFormat)
	future := stat.ModTime().Add(time.Hour).UTC().Format(http.TimeFormat)

	index, server := makeServer(t)
	defer server.Shutdown()
	defer index.Close()

	type testCase struct {
		Status            int
		IfModifiedSince   string
		IfUnmodifiedSince string
	}
	tests := []testCase{
		{200, "", ""},
		{200, "junk", ""},
		{200, "", "junk"},
		{200, "junk", "junk"},

		// RFC 7232, section 6, condition 4
		{200, past, ""},
		{304, present, ""},
		{304, future, ""},

		// RFC 7232, section 6, condition 2
		{412, "", past},
		{200, "", present},
		{200, "", future},
	}
	for _, e := range tests {
		query, _ := http.NewRequest("GET", "/collections/castles/items", nil)
		if len(e.IfModifiedSince) > 0 {
			query.Header.Add("If-Modified-Since", e.IfModifiedSince)
		}
		if len(e.IfUnmodifiedSince) > 0 {
			query.Header.Add("If-Unmodified-Since", e.IfUnmodifiedSince)
		}
		handler := http.HandlerFunc(server.HandleRequest)
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, query)
		if status := resp.Result().StatusCode; status != e.Status {
			t.Errorf("expected %d for If-ModifiedSince: %s / If-Unmodified-Since: %s; got %d",
				e.Status, e.IfModifiedSince, e.IfUnmodifiedSince, status)
		}
	}
}

func TestItem(t *testing.T) {
	index, s := makeServer(t)
	defer s.Shutdown()
	defer index.Close()
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

func TestTilesFeatureInfo(t *testing.T) {
	index, s := makeServer(t)
	defer s.Shutdown()
	defer index.Close()
	query, _ := http.NewRequest("GET", "/tiles/castles/17/69585/46595/102/50.geojson", nil)
	handler := http.HandlerFunc(s.HandleRequest)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, query)
	if ct := resp.Header().Get("Content-Type"); ct != "application/geo+json" {
		t.Errorf("Expected Content-Type: application/geo+json, got %s", ct)
	}
	expectCORSHeader(t, resp.Header())
	expectJSON(t, getBody(resp), `{
          "type": "FeatureCollection",
          "features": [
            {
              "id": "W24785843",
              "type": "Feature",
              "geometry": {
                "type": "Polygon",
                "coordinates": [
                  [
                    [
                      11.1221624,
                      46.0670118
                    ],
                    [
                      11.1221546,
                      46.0670507
                    ],
                    [
                      11.1221723,
                      46.0670574
                    ],
                    [
                      11.1221842,
                      46.0670731
                    ],
                    [
                      11.1221869,
                      46.067097
                    ],
                    [
                      11.1221766,
                      46.0671192
                    ],
                    [
                      11.1221453,
                      46.067136
                    ],
                    [
                      11.1221161,
                      46.0671393
                    ],
                    [
                      11.1220222,
                      46.0674756
                    ],
                    [
                      11.1219216,
                      46.0674735
                    ],
                    [
                      11.1219202,
                      46.0675053
                    ],
                    [
                      11.1218793,
                      46.0675034
                    ],
                    [
                      11.1218347,
                      46.0675014
                    ],
                    [
                      11.1218655,
                      46.0672963
                    ],
                    [
                      11.1218916,
                      46.0670783
                    ],
                    [
                      11.1218991,
                      46.0669992
                    ],
                    [
                      11.1220515,
                      46.067004
                    ],
                    [
                      11.1221624,
                      46.0670118
                    ]
                  ]
                ]
              },
              "properties": {
                "building": "yes",
                "historic": "castle",
                "name": "Palazzo Pretorio",
                "wikidata": "Q26997946"
              }
            }
          ],
          "bbox": [
            11.1218347,
            46.0669992,
            11.1221869,
            46.0675053
          ]
	}`)
}

func TestTilesFeatureInfo_NoFeatures(t *testing.T) {
	index, s := makeServer(t)
	defer s.Shutdown()
	defer index.Close()
	query, _ := http.NewRequest("GET", "/tiles/castles/17/69585/46595/10/5.geojson", nil)
	handler := http.HandlerFunc(s.HandleRequest)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, query)
	if ct := resp.Header().Get("Content-Type"); ct != "application/geo+json" {
		t.Errorf("Expected Content-Type: application/geo+json, got %s", ct)
	}
	expectCORSHeader(t, resp.Header())
	expectJSON(t, getBody(resp), `{
          "type": "FeatureCollection",
          "features": [
          ],
	  "bbox": null
        }`)
}

func TestTilesFeatureInfo_NotFound(t *testing.T) {
	index, s := makeServer(t)
	defer s.Shutdown()
	defer index.Close()
	query, _ := http.NewRequest("GET", "/tiles/nosuchcollection/1/2/3.geojson", nil)
	handler := http.HandlerFunc(s.HandleRequest)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, query)
	r := resp.Result()
	if r.StatusCode != http.StatusNotFound {
		t.Errorf("expected %d, got %d", http.StatusNotFound, r.StatusCode)
	}
}

func expectCORSHeader(t *testing.T, header http.Header) {
	if cors := header.Get("Access-Control-Allow-Origin"); cors != "*" {
		t.Errorf("expected header \"Access-Control-Allow-Origin: *\", got %s", cors)
	}
}
