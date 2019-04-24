package main

import (
	"bytes"
	"github.com/golang/geo/r2"
	"image/png"
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func BenchmarkTile0Points(b *testing.B) {
	benchmarkTileNPoints(b, 0)
}

func BenchmarkTile10Points(b *testing.B) {
	benchmarkTileNPoints(b, 10)
}

func BenchmarkTile100Points(b *testing.B) {
	benchmarkTileNPoints(b, 100)
}

func benchmarkTileNPoints(b *testing.B, n int) {
	rnd := rand.New(rand.NewSource(12345))
	points := make([]r2.Point, n)
	for i := 0; i < len(points); i++ {
		points[i].X = math.Mod(rnd.Float64(), 256.0+20.0) - 10.0
		points[i].Y = math.Mod(rnd.Float64(), 256.0+20.0) - 10.0
	}
	for i := 0; i < b.N; i++ {
		var tile Tile
		for _, p := range points {
			tile.DrawPoint(p)
		}
		tile.ToPNG()
	}
}

func BenchmarkTileCacheGet(b *testing.B) {
	tc := NewTileCache(10000)
	key := TileKey{Zoom: 0, X: 12, Y: 7}
	tc.Put(key, []byte("cached content"))
	for i := 0; i < b.N; i++ {
		tc.Get(key)
	}
}

func TestEmptyPNG(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(emptyPNG))
	if err != nil {
		t.Fatal(err)
	}
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			_, _, _, alpha := img.At(x, y).RGBA()
			if alpha != 0 {
				t.Errorf("expected transparent pixel at (%d, %d), got alpha %d",
					x, y, alpha)
			}
		}
	}
}

func TestTile_Empty(t *testing.T) {
	var tile Tile
	if !reflect.DeepEqual(tile.ToPNG(), emptyPNG) {
		t.Error("expected pre-compressed empty tile")
	}
}

func TestTile_DrawPoint(t *testing.T) {
	var tile Tile
	tile.DrawPoint(r2.Point{7.02, 22.95})
	img, err := png.Decode(bytes.NewReader(tile.ToPNG()))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, alpha := img.At(7, 23).RGBA(); alpha != 0xFFFF {
		t.Errorf("expected opaque pixel at (%d, %d), got alpha %d",
			7, 23, alpha)
	}
	if _, _, _, alpha := img.At(66, 207).RGBA(); alpha != 0 {
		t.Errorf("expected transparent pixel at (%d, %d), got alpha %d",
			66, 207, alpha)
	}
}

func TestTileCache(t *testing.T) {
	foo := []byte("foo")
	bar := []byte("bar")
	cache := NewTileCache(2)
	key := TileKey{Zoom: 0, X: 12, Y: 7}
	if v := cache.Get(key); v != nil {
		t.Errorf("expected nil, got %s", string(v))
	}
	if cache.size != 0 {
		t.Errorf("expected size 0, got %d", cache.size)
	}
	cache.Put(key, foo)
	if cache.size != 1 {
		t.Errorf("expected size 1, got %d", cache.size)
	}
	if v := cache.Get(key); !reflect.DeepEqual(v, foo) {
		t.Errorf("expected foo, got %s", string(v))
	}
	cache.Put(key, bar)
	if cache.size != 1 {
		t.Errorf("expected size 1, got %d", cache.size)
	}
	if v := cache.Get(key); !reflect.DeepEqual(v, bar) {
		t.Errorf("expected bar, got %s", string(v))
	}
	cache.Put(TileKey{Zoom: 11, X: 80, Y: 91}, foo)
	cache.Put(TileKey{Zoom: 11, X: 90, Y: 81}, foo)
	if cache.size != 2 {
		t.Errorf("expected size 2, got %d", cache.size)
	}
}
