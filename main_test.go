package main

import (
	"image"
	"image/color"
	"testing"
)

// Helper function to create a blank test image
func createTestImage(width, height int, baseColor color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, baseColor)
		}
	}
	return img
}

func TestAnalyzeFrame(t *testing.T) {
	threshold := 10

	t.Run("both absent (black background)", func(t *testing.T) {
		img := createTestImage(100, 100, color.Black)
		res := AnalyzeFrame(img, threshold)
		if res.FireDetected {
			t.Errorf("expected no fire, but got fire detected")
		}
		if res.BlueLightDetected {
			t.Errorf("expected no blue light, but got blue light detected")
		}
	})

	t.Run("fire present", func(t *testing.T) {
		img := createTestImage(100, 100, color.Black)
		// Set pixels matching fire condition: R > 200, G > 100, B < 120, R > G > B
		fireColor := color.RGBA{R: 240, G: 150, B: 50, A: 255}
		for i := 0; i < threshold; i++ {
			img.Set(i, 0, fireColor)
		}
		res := AnalyzeFrame(img, threshold)
		if !res.FireDetected {
			t.Errorf("expected fire detected, but got false")
		}
		if res.BlueLightDetected {
			t.Errorf("expected no blue light, but got blue light detected")
		}
	})

	t.Run("blue light present", func(t *testing.T) {
		img := createTestImage(100, 100, color.Black)
		// Set pixels matching blue light condition: B > 150, B > R+60, B > G+60
		blueColor := color.RGBA{R: 50, G: 50, B: 240, A: 255}
		for i := 0; i < threshold; i++ {
			img.Set(i, 0, blueColor)
		}
		res := AnalyzeFrame(img, threshold)
		if res.FireDetected {
			t.Errorf("expected no fire, but got fire detected")
		}
		if !res.BlueLightDetected {
			t.Errorf("expected blue light detected, but got false")
		}
	})

	t.Run("both present", func(t *testing.T) {
		img := createTestImage(100, 100, color.Black)
		fireColor := color.RGBA{R: 240, G: 150, B: 50, A: 255}
		blueColor := color.RGBA{R: 50, G: 50, B: 240, A: 255}
		for i := 0; i < threshold; i++ {
			img.Set(i, 0, fireColor)
			img.Set(i, 1, blueColor)
		}
		res := AnalyzeFrame(img, threshold)
		if !res.FireDetected {
			t.Errorf("expected fire detected, but got false")
		}
		if !res.BlueLightDetected {
			t.Errorf("expected blue light detected, but got false")
		}
	})
}
