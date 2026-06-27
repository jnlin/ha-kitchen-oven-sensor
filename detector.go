package main

import (
	"image"
)

// DetectionResult holds the results of the frame analysis.
type DetectionResult struct {
	FireDetected      bool
	BlueLightDetected bool
	FirePixelCount    int
	BluePixelCount    int
}

// AnalyzeFrame processes the image to check for fire/flames and blue light source.
// It returns a DetectionResult.
func AnalyzeFrame(img image.Image, threshold int) DetectionResult {
	bounds := img.Bounds()
	firePixels := 0
	bluePixels := 0

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)
			r, g, b, _ := c.RGBA()
			// RGBA() returns values in the range [0, 65535]. We scale to [0, 255] for standard logic.
			r8 := r >> 8
			g8 := g >> 8
			b8 := b >> 8

			if isFirePixel(r8, g8, b8) {
				firePixels++
			}
			if isBlueLightPixel(r8, g8, b8) {
				bluePixels++
			}
		}
	}

	return DetectionResult{
		FireDetected:      firePixels >= threshold,
		BlueLightDetected: bluePixels >= threshold,
		FirePixelCount:    firePixels,
		BluePixelCount:    bluePixels,
	}
}

// isFirePixel checks if a pixel color matches standard fire/flame thresholds.
// Fire is bright, with high red component, and red > green > blue.
func isFirePixel(r, g, b uint32) bool {
	return r > 200 && g > 100 && b < 120 && r > g && g > b
}

// isBlueLightPixel checks if a pixel color matches standard blue light thresholds.
// Blue is high and significantly higher than red and green.
func isBlueLightPixel(r, g, b uint32) bool {
	return b > 150 && b > r+60 && b > g+60
}
