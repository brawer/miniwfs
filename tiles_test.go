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
