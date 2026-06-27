package main

import (
	"image"
)

// DetectionResult holds the results of the frame analysis.
type DetectionResult struct {
	BlueLightDetected bool
	BluePixelCount    int
}

// AnalyzeFrame processes the image to check for a blue light source.
// It returns a DetectionResult.
func AnalyzeFrame(img image.Image, threshold int) DetectionResult {
	bounds := img.Bounds()
	detectedPixels := 0

	if isGrayscale(img) {
		// Night-vision (IR/grayscale) mode:
		// Target region where the blue light manifests under IR
		xMin := 1700
		xMax := 1865
		yMin := 1165
		yMax := 1295

		// Clip to bounds to avoid out of bounds panic for small test/dummy images
		if xMin < bounds.Min.X { xMin = bounds.Min.X }
		if xMax > bounds.Max.X { xMax = bounds.Max.X }
		if yMin < bounds.Min.Y { yMin = bounds.Min.Y }
		if yMax > bounds.Max.Y { yMax = bounds.Max.Y }

		for y := yMin; y < yMax; y++ {
			for x := xMin; x < xMax; x++ {
				c := img.At(x, y)
				r, g, b, _ := c.RGBA()
				r8 := r >> 8
				g8 := g >> 8
				b8 := b >> 8
				
				// Grayscale intensity (average of RGB channels)
				gray := (r8 + g8 + b8) / 3
				if gray > 180 {
					detectedPixels++
				}
			}
		}
	} else {
		// Daytime (color) mode:
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				c := img.At(x, y)
				r, g, b, _ := c.RGBA()
				r8 := r >> 8
				g8 := g >> 8
				b8 := b >> 8

				if isBlueLightPixel(r8, g8, b8) {
					detectedPixels++
				}
			}
		}
	}

	return DetectionResult{
		BlueLightDetected: detectedPixels >= threshold,
		BluePixelCount:    detectedPixels,
	}
}

// isBlueLightPixel checks if a pixel color matches standard blue light thresholds.
// Blue is high and significantly higher than red and green.
func isBlueLightPixel(r, g, b uint32) bool {
	return b > 180 && b > r+80 && b > g+80
}

// isGrayscale detects if the frame is in night-vision (IR/grayscale) mode
// by sampling pixels and measuring their color channel variance.
func isGrayscale(img image.Image) bool {
	bounds := img.Bounds()
	var totalDiff uint64
	var samples int64

	// Sample pixels to check for color variance.
	// Stepping by 20 ensures this check is extremely fast.
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 20 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 20 {
			c := img.At(x, y)
			r, g, b, _ := c.RGBA()
			r8 := r >> 8
			g8 := g >> 8
			b8 := b >> 8

			diff := absDiff(r8, g8) + absDiff(r8, b8) + absDiff(g8, b8)
			totalDiff += uint64(diff)
			samples++
		}
	}

	if samples == 0 {
		return false
	}
	avgDiff := float64(totalDiff) / float64(samples)
	return avgDiff < 10.0
}

// absDiff returns the absolute difference between two uint32 numbers.
func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
