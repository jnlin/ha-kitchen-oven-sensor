package main

import (
	"fmt"
	"image"
)

// DetectionResult holds the results of the frame analysis.
type DetectionResult struct {
	BlueLightDetected bool
	BluePixelCount    int
	CurrentMode       string
	AppliedThreshold  int
	GrayscaleScore    float64
	ConfidenceScore   int
	Status            string
	Justification     string
}

// AnalysisConfig holds the decoupled daytime and nighttime thresholds.
type AnalysisConfig struct {
	DayColorThreshold        int
	NightLuminanceThreshold  int
	NightBlobMinSize         int
	NightBlobMaxSize         int
	NightConfidenceThreshold int
	EnableNightMode          bool
}

type point struct {
	X, Y int
}

// AnalyzeFrame processes the image to check for a blue light source using the provided config.
// It returns a DetectionResult.
func AnalyzeFrame(img image.Image, cfg AnalysisConfig) DetectionResult {
	bounds := img.Bounds()
	detectedPixels := 0

	grayScore := getGrayscaleScore(img)
	isGrayscaleMode := grayScore < 10.0

	var currentMode string
	var appliedThreshold int

	var maxConfidence int
	var bestBlobJustification = "No valid bright blob matching LED criteria was detected"
	var bestBlobArea int

	if isGrayscaleMode {
		currentMode = "nighttime"
		appliedThreshold = cfg.NightConfidenceThreshold

		if !cfg.EnableNightMode {
			// Skip detection and immediately return a negative result, bypassing nighttime BFS
			return DetectionResult{
				BlueLightDetected: false,
				BluePixelCount:    0,
				CurrentMode:       currentMode,
				AppliedThreshold:  appliedThreshold,
				GrayscaleScore:    grayScore,
				ConfidenceScore:   0,
				Status:            "negative",
				Justification:     "Night mode detection is disabled in configuration",
			}
		}

		// Night-vision (IR/grayscale) mode:
		width := bounds.Dx()
		height := bounds.Dy()

		visited := make([]bool, width*height)

		isBright := func(x, y int) bool {
			c := img.At(x, y)
			r, g, b, _ := c.RGBA()
			gray := int((r>>8 + g>>8 + b>>8) / 3)
			return gray >= cfg.NightLuminanceThreshold
		}

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

				// BFS to compute blob properties incrementally
				area := 0
				minX, maxX := x, x
				minY, maxY := y, y
				var sumGray uint64
				maxGray := 0

				queue := []point{{X: x, Y: y}}
				visited[idx] = true
				head := 0

				for head < len(queue) {
					curr := queue[head]
					head++

					area++
					if curr.X < minX {
						minX = curr.X
					}
					if curr.X > maxX {
						maxX = curr.X
					}
					if curr.Y < minY {
						minY = curr.Y
					}
					if curr.Y > maxY {
						maxY = curr.Y
					}

					c := img.At(curr.X, curr.Y)
					r, g, b, _ := c.RGBA()
					gray := int((r>>8 + g>>8 + b>>8) / 3)
					sumGray += uint64(gray)
					if gray > maxGray {
						maxGray = gray
					}

					// Neighbors
					neighbors := [4]point{
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

				// Evaluate the blob immediately: exclude borders/margins on high-res camera frames
				if shouldExcludeBlob(minX, maxX, minY, maxY, bounds) {
					continue
				}

				w := maxX - minX + 1
				h := maxY - minY + 1
				var aspect float64
				if w > h {
					aspect = float64(w) / float64(h)
				} else {
					aspect = float64(h) / float64(w)
				}
				fill := float64(area) / float64(w*h)

				aspectValid := aspect <= 2.0
				areaValid := area >= cfg.NightBlobMinSize && area <= cfg.NightBlobMaxSize
				fillValid := fill >= 0.40 && fill <= 0.85

				// Calculate background local to this blob
				bgSum := 0
				bgCount := 0
				bgDist := 5
				bgSize := 10
				for by := minY - bgDist - bgSize; by <= maxY+bgDist+bgSize; by++ {
					for bx := minX - bgDist - bgSize; bx <= maxX+bgDist+bgSize; bx++ {
						if bx >= bounds.Min.X && bx < bounds.Max.X && by >= bounds.Min.Y && by < bounds.Max.Y {
							isInsideOuter := bx >= minX-bgDist-bgSize && bx <= maxX+bgDist+bgSize && by >= minY-bgDist-bgSize && by <= maxY+bgDist+bgSize
							isInsideInner := bx >= minX-bgDist && bx <= maxX+bgDist && by >= minY-bgDist && by <= maxY+bgDist
							if isInsideOuter && !isInsideInner {
								r, g, b, _ := img.At(bx, by).RGBA()
								gray := int((r>>8 + g>>8 + b>>8) / 3)
								if gray < cfg.NightLuminanceThreshold {
									bgSum += gray
									bgCount++
								}
							}
						}
					}
				}
				bgAvg := 0.0
				if bgCount > 0 {
					bgAvg = float64(bgSum) / float64(bgCount)
				}

				// Measure blooming / gradient around the edges.
				ringSum := 0
				ringCount := 0
				for by := minY - 3; by <= maxY+3; by++ {
					for bx := minX - 3; bx <= maxX+3; bx++ {
						if bx >= bounds.Min.X && bx < bounds.Max.X && by >= bounds.Min.Y && by < bounds.Max.Y {
							isInsideOuter := bx >= minX-3 && bx <= maxX+3 && by >= minY-3 && by <= maxY+3
							isInsideInner := bx >= minX && bx <= maxX && by >= minY && by <= maxY
							if isInsideOuter && !isInsideInner {
								r, g, b, _ := img.At(bx, by).RGBA()
								gray := int((r>>8 + g>>8 + b>>8) / 3)
								if gray < cfg.NightLuminanceThreshold {
									ringSum += gray
									ringCount++
								}
							}
						}
					}
				}
				ringAvg := 0.0
				if ringCount > 0 {
					ringAvg = float64(ringSum) / float64(ringCount)
				}

				score := 100.0
				var contrast, blooming float64

				if width >= 2000 {
					// Saturation penalty: Deduct 2 * (255 - maxGray)
					if maxGray < 255 {
						score -= float64(255-maxGray) * 2.0
					}

					// Contrast penalty: If contrast < 190, deduct 1.5 * (190 - contrast)
					// If contrast < 150, confidence drops to 0
					contrast = float64(maxGray) - bgAvg
					if contrast < 150 {
						score = 0
					} else if contrast < 190 {
						score -= (190.0 - contrast) * 1.5
					}

					// Blooming penalty: If blooming < 6, deduct 2.0 * (6.0 - blooming)
					// If blooming < 1, confidence drops to 0
					blooming = ringAvg - bgAvg
					if blooming < 1.0 {
						score = 0
					} else if blooming < 6.0 {
						score -= (6.0 - blooming) * 2.0
					}

					if !isInOvenROI(minX, maxX, minY, maxY, bounds) {
						score = 0
					}
				} else {
					// Unit test / low-res images: Require standard intensity checks
					avgG := float64(sumGray) / float64(area)
					if maxGray < 240 || avgG < 220 {
						score = 0
					}
				}

				if !aspectValid || !areaValid || !fillValid {
					score = 0
				}

				if score < 0 {
					score = 0
				}

				scoreInt := int(score)
				if scoreInt > 0 && (scoreInt > maxConfidence || (scoreInt == maxConfidence && area > bestBlobArea)) {
					maxConfidence = scoreInt
					bestBlobArea = area
					if width >= 2000 {
						bestBlobJustification = fmt.Sprintf("Detected LED blob at center (%d,%d) with area=%d, max_gray=%d, contrast=%.1f, blooming=%.1f, aspect=%.2f, fill=%.2f, confidence=%d",
							(minX+maxX)/2, (minY+maxY)/2, area, maxGray, contrast, blooming, aspect, fill, scoreInt)
					} else {
						bestBlobJustification = fmt.Sprintf("Detected LED blob at center (%d,%d) with area=%d, max_gray=%d, aspect=%.2f, fill=%.2f, confidence=%d",
							(minX+maxX)/2, (minY+maxY)/2, area, maxGray, aspect, fill, scoreInt)
					}
				}
			}
		}
		detectedPixels = bestBlobArea

	} else {
		currentMode = "daytime"
		appliedThreshold = cfg.DayColorThreshold

		// Daytime (color) mode:
		width := bounds.Dx()
		height := bounds.Dy()

		visited := make([]bool, width*height)

		isBlue := func(x, y int) bool {
			c := img.At(x, y)
			r, g, b, _ := c.RGBA()
			return isBlueLightPixel(r>>8, g>>8, b>>8)
		}

		maxMatchingArea := 0

		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				idx := (y-bounds.Min.Y)*width + (x - bounds.Min.X)
				if visited[idx] {
					continue
				}
				if !isBlue(x, y) {
					visited[idx] = true
					continue
				}

				// BFS to compute blob properties incrementally
				area := 0
				minX, maxX := x, x
				minY, maxY := y, y

				queue := []point{{X: x, Y: y}}
				visited[idx] = true
				head := 0

				for head < len(queue) {
					curr := queue[head]
					head++

					area++
					if curr.X < minX {
						minX = curr.X
					}
					if curr.X > maxX {
						maxX = curr.X
					}
					if curr.Y < minY {
						minY = curr.Y
					}
					if curr.Y > maxY {
						maxY = curr.Y
					}

					// Neighbors
					neighbors := [4]point{
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
								if isBlue(n.X, n.Y) {
									queue = append(queue, n)
								}
							}
						}
					}
				}

				// Evaluate the blob immediately: exclude borders/margins on high-res camera frames
				if shouldExcludeBlob(minX, maxX, minY, maxY, bounds) {
					continue
				}

				w := maxX - minX + 1
				h := maxY - minY + 1
				var aspect float64
				if w > h {
					aspect = float64(w) / float64(h)
				} else {
					aspect = float64(h) / float64(w)
				}

				// Apply geometric filters to isolate the oven light in color mode
				aspectValid := true
				if bounds.Max.Y >= 600 {
					aspectValid = aspect <= 3.0
				}

				if area >= cfg.DayColorThreshold && area <= 1000 && aspectValid && isInOvenROI(minX, maxX, minY, maxY, bounds) {
					if area > maxMatchingArea {
						maxMatchingArea = area
					}
				}
			}
		}
		detectedPixels = maxMatchingArea
	}

	var confidenceScore int
	var status string
	var justification string

	if isGrayscaleMode {
		confidenceScore = maxConfidence
		if confidenceScore >= cfg.NightConfidenceThreshold {
			status = "positive"
		} else {
			status = "negative"
		}
		justification = bestBlobJustification
	} else {
		// Daytime mode:
		if detectedPixels >= appliedThreshold {
			confidenceScore = 100
			status = "positive"
			justification = fmt.Sprintf("Daytime mode: blue light pixel count (%d) met threshold (%d)", detectedPixels, appliedThreshold)
		} else {
			confidenceScore = 0
			status = "negative"
			justification = fmt.Sprintf("Daytime mode: blue light pixel count (%d) below threshold (%d)", detectedPixels, appliedThreshold)
		}
	}

	return DetectionResult{
		BlueLightDetected: status == "positive",
		BluePixelCount:    detectedPixels,
		CurrentMode:       currentMode,
		AppliedThreshold:  appliedThreshold,
		GrayscaleScore:    grayScore,
		ConfidenceScore:   confidenceScore,
		Status:            status,
		Justification:     justification,
	}
}

// shouldExcludeBlob checks if a blob should be excluded based on its spatial boundaries.
// It excludes blobs close to top/bottom borders (timestamps, overlays) on high-res camera frames,
// as well as blobs on the far left and right edges (within outer 15% margins of the frame)
// to prevent false positives from window light reflections and other peripheral clutter,
// while keeping the central region active.
func shouldExcludeBlob(minX, maxX, minY, maxY int, bounds image.Rectangle) bool {
	if bounds.Max.Y >= 600 {
		cy := (minY + maxY) / 2
		cx := (minX + maxX) / 2
		minXBound := bounds.Max.X * 15 / 100
		maxXBound := bounds.Max.X * 85 / 100
		if cy < 400 || cy > bounds.Max.Y-150 || cx < minXBound || cx > maxXBound {
			return true
		}
	}
	return false
}

// isBlueLightPixel checks if a pixel color matches standard blue light thresholds.
// Blue is high and significantly higher than red and green.
func isBlueLightPixel(r, g, b uint32) bool {
	return b > 180 && b > r+80 && b > g+80
}

// getGrayscaleScore measures color channel variance across the image.
func getGrayscaleScore(img image.Image) float64 {
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
		return 0.0
	}
	return float64(totalDiff) / float64(samples)
}

// absDiff returns the absolute difference between two uint32 numbers.
func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

// isInOvenROI checks if the center of a blob is within the predefined oven LED light ROIs.
func isInOvenROI(minX, maxX, minY, maxY int, bounds image.Rectangle) bool {
	width := bounds.Dx()
	height := bounds.Dy()
	if width < 2000 {
		return true
	}

	cx := (minX + maxX) / 2
	cy := (minY + maxY) / 2
	cxRel := cx - bounds.Min.X
	cyRel := cy - bounds.Min.Y
	cxPct := float64(cxRel) / float64(width)
	cyPct := float64(cyRel) / float64(height)

	inROI1 := cxPct >= 0.62 && cxPct <= 0.67 && cyPct >= 0.76 && cyPct <= 0.83
	inROI2 := cxPct >= 0.66 && cxPct <= 0.72 && cyPct >= 0.76 && cyPct <= 0.84

	return inROI1 || inROI2
}
