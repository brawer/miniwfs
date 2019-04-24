package main

import (
	"bytes"
	"github.com/golang/geo/r2"
	"image/png"
	"reflect"
	"testing"
)

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
