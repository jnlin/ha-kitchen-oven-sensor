package main

import (
	"image"
)

// DetectionResult holds the results of the frame analysis.
type DetectionResult struct {
	BlueLightDetected bool
	BluePixelCount    int
}

type point struct {
	X, Y int
}

type blob struct {
	minX, maxX int
	minY, maxY int
	points     []point
	maxGray    int
	avgGray    float64
}

func (b *blob) area() int { return len(b.points) }
func (b *blob) width() int { return b.maxX - b.minX + 1 }
func (b *blob) height() int { return b.maxY - b.minY + 1 }
func (b *blob) aspectRatio() float64 {
	w, h := float64(b.width()), float64(b.height())
	if w > h {
		return w / h
	}
	return h / w
}
func (b *blob) fillRatio() float64 {
	return float64(len(b.points)) / float64(b.width()*b.height())
}

// AnalyzeFrame processes the image to check for a blue light source.
// It returns a DetectionResult.
func AnalyzeFrame(img image.Image, threshold int) DetectionResult {
	bounds := img.Bounds()
	detectedPixels := 0

	if isGrayscale(img) {
		// Night-vision (IR/grayscale) mode:
		width := bounds.Dx()
		height := bounds.Dy()

		visited := make([]bool, width*height)

		isBright := func(x, y int) bool {
			c := img.At(x, y)
			r, g, b, _ := c.RGBA()
			gray := int((r >> 8 + g >> 8 + b >> 8) / 3)
			return gray >= 180
		}

		var blobs []blob

		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				idx := (y-bounds.Min.Y)*width + (x - bounds.Min.X)
				if visited[idx] {
					continue
				}
				if !isBright(x, y) {
					visited[idx] = true
					continue
				}

				var blobPoints []point
				minX, maxX := x, x
				minY, maxY := y, y

				queue := []point{{X: x, Y: y}}
				visited[idx] = true

				for len(queue) > 0 {
					curr := queue[0]
					queue = queue[1:]
					blobPoints = append(blobPoints, curr)

					if curr.X < minX { minX = curr.X }
					if curr.X > maxX { maxX = curr.X }
					if curr.Y < minY { minY = curr.Y }
					if curr.Y > maxY { maxY = curr.Y }

					neighbors := []point{
						{X: curr.X + 1, Y: curr.Y},
						{X: curr.X - 1, Y: curr.Y},
						{X: curr.X, Y: curr.Y + 1},
						{X: curr.X, Y: curr.Y - 1},
					}

					for _, n := range neighbors {
						if n.X >= bounds.Min.X && n.X < bounds.Max.X && n.Y >= bounds.Min.Y && n.Y < bounds.Max.Y {
							nIdx := (n.Y-bounds.Min.Y)*width + (n.X - bounds.Min.X)
							if !visited[nIdx] {
								visited[nIdx] = true
								if isBright(n.X, n.Y) {
									queue = append(queue, n)
								}
							}
						}
					}
				}

				var sum uint64
				maxG := 0
				for _, pt := range blobPoints {
					c := img.At(pt.X, pt.Y)
					r, g, b, _ := c.RGBA()
					gray := int((r >> 8 + g >> 8 + b >> 8) / 3)
					sum += uint64(gray)
					if gray > maxG {
						maxG = gray
					}
				}

				blobs = append(blobs, blob{
					minX:    minX,
					maxX:    maxX,
					minY:    minY,
					maxY:    maxY,
					points:  blobPoints,
					maxGray: maxG,
					avgGray: float64(sum) / float64(len(blobPoints)),
				})
			}
		}

		maxMatchingArea := 0
		for _, b := range blobs {
			cy := (b.minY + b.maxY) / 2

			// Exclude blobs close to top/bottom borders (timestamps, overlays)
			if cy < 400 || cy > bounds.Max.Y-150 {
				continue
			}

			area := b.area()
			aspect := b.aspectRatio()
			fill := b.fillRatio()

			// Apply geometric and intensity filters to find the oven light source
			if area >= 80 && area <= 400 && aspect <= 2.5 && fill >= 0.40 && b.maxGray >= 240 && b.avgGray >= 210 {
				if area > maxMatchingArea {
					maxMatchingArea = area
				}
			}
		}
		detectedPixels = maxMatchingArea

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
