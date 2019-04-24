package main

import (
	"bytes"
	"image/png"
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
