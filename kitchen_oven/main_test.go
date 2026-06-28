package main

import (
	"encoding/json"
	"image"
	"image/color"
	_ "image/jpeg"
	"os"
	"path/filepath"
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
	cfg := AnalysisConfig{
		DayColorThreshold:        10,
		NightLuminanceThreshold:  180,
		NightBlobMinSize:         80,
		NightBlobMaxSize:         1000,
		NightConfidenceThreshold: 80,
		EnableNightMode:          true,
	}

	t.Run("both absent (black background)", func(t *testing.T) {
		img := createTestImage(100, 100, color.Black)
		res := AnalyzeFrame(img, cfg)
		if res.BlueLightDetected {
			t.Errorf("expected no blue light, but got blue light detected")
		}
	})

	t.Run("fire present (should be negative)", func(t *testing.T) {
		img := createTestImage(100, 100, color.Black)
		// Set pixels matching fire color (which is NOT blue light)
		fireColor := color.RGBA{R: 240, G: 150, B: 50, A: 255}
		for i := 0; i < cfg.DayColorThreshold; i++ {
			img.Set(i, 0, fireColor)
		}
		res := AnalyzeFrame(img, cfg)
		if res.BlueLightDetected {
			t.Errorf("expected no blue light, but got blue light detected")
		}
	})

	t.Run("blue light present", func(t *testing.T) {
		img := createTestImage(100, 100, color.Black)
		// Set pixels matching blue light condition: B > 180, B > R+80, B > G+80
		blueColor := color.RGBA{R: 50, G: 50, B: 240, A: 255}
		for i := 0; i < cfg.DayColorThreshold; i++ {
			img.Set(i, 0, blueColor)
		}
		res := AnalyzeFrame(img, cfg)
		if !res.BlueLightDetected {
			t.Errorf("expected blue light detected, but got false")
		}
	})

	t.Run("both present (should detect blue light)", func(t *testing.T) {
		img := createTestImage(100, 100, color.Black)
		fireColor := color.RGBA{R: 240, G: 150, B: 50, A: 255}
		blueColor := color.RGBA{R: 50, G: 50, B: 240, A: 255}
		for i := 0; i < cfg.DayColorThreshold; i++ {
			img.Set(i, 0, fireColor)
			img.Set(i, 1, blueColor)
		}
		res := AnalyzeFrame(img, cfg)
		if !res.BlueLightDetected {
			t.Errorf("expected blue light detected, but got false")
		}
	})

	t.Run("margin exclusion on high-res frame", func(t *testing.T) {
		// Image bounds: 1000x800.
		// Margins: Y: Top 400 pixels are excluded (cy < 400), bottom 150 pixels are excluded (cy > 800 - 150 = 650).
		//          X: Left 150 pixels are excluded (cx < 15% of 1000 = 150), right 150 pixels are excluded (cx > 85% of 1000 = 850).
		// Center: (150 to 850, 400 to 650)

		// 1. Daytime (color) mode testing
		// We'll set a blue blob (matching dayColorThreshold = 10) in various locations.
		blueColor := color.RGBA{R: 50, G: 50, B: 240, A: 255}
		redBackground := color.RGBA{R: 200, G: 0, B: 0, A: 255}

		// 1a. Blue blob in far-left excluded region (X: 50..53, Y: 500..502)
		imgLeft := createTestImage(1000, 800, redBackground)
		for y := 500; y < 503; y++ {
			for x := 50; x < 54; x++ {
				imgLeft.Set(x, y, blueColor)
			}
		}
		res := AnalyzeFrame(imgLeft, cfg)
		if res.BlueLightDetected {
			t.Errorf("daytime: expected no detection in left margin, but got true")
		}

		// 1b. Blue blob in far-right excluded region (X: 900..903, Y: 500..502)
		imgRight := createTestImage(1000, 800, redBackground)
		for y := 500; y < 503; y++ {
			for x := 900; x < 904; x++ {
				imgRight.Set(x, y, blueColor)
			}
		}
		res = AnalyzeFrame(imgRight, cfg)
		if res.BlueLightDetected {
			t.Errorf("daytime: expected no detection in right margin, but got true")
		}

		// 1c. Blue blob in active center region (X: 500..503, Y: 500..502)
		imgCenter := createTestImage(1000, 800, redBackground)
		for y := 500; y < 503; y++ {
			for x := 500; x < 504; x++ {
				imgCenter.Set(x, y, blueColor)
			}
		}
		res = AnalyzeFrame(imgCenter, cfg)
		if !res.BlueLightDetected {
			t.Errorf("daytime: expected detection in center region, but got false")
		}

		// 2. Nighttime (grayscale) mode testing
		// We'll set a bright blob (matching nightBlobMinSize = 80) in various locations.
		// For grayscale mode, we need:
		// - raw gray >= NightLuminanceThreshold (180)
		// - maxGray >= 240
		// - avgG >= 220
		// - fill >= 0.40 && fill <= 0.75
		// - aspect <= 2.0
		// We'll construct a circular blob of radius 5. Area is 81 pixels, aspect is 1.0, fill is 0.67, avgG is 255.
		brightColor := color.Gray{Y: 255}

		// 2a. Bright blob in far-left excluded region (center X = 55, center Y = 505)
		imgNightLeft := createTestImage(1000, 800, color.Black)
		for dy := -5; dy <= 5; dy++ {
			for dx := -5; dx <= 5; dx++ {
				if dx*dx+dy*dy <= 25 {
					imgNightLeft.Set(55+dx, 505+dy, brightColor)
				}
			}
		}
		res = AnalyzeFrame(imgNightLeft, cfg)
		if res.BlueLightDetected {
			t.Errorf("nighttime: expected no detection in left margin, but got true")
		}

		// 2b. Bright blob in far-right excluded region (center X = 905, center Y = 505)
		imgNightRight := createTestImage(1000, 800, color.Black)
		for dy := -5; dy <= 5; dy++ {
			for dx := -5; dx <= 5; dx++ {
				if dx*dx+dy*dy <= 25 {
					imgNightRight.Set(905+dx, 505+dy, brightColor)
				}
			}
		}
		res = AnalyzeFrame(imgNightRight, cfg)
		if res.BlueLightDetected {
			t.Errorf("nighttime: expected no detection in right margin, but got true")
		}

		// 2c. Bright blob in active center region (center X = 505, center Y = 505)
		imgNightCenter := createTestImage(1000, 800, color.Black)
		for dy := -5; dy <= 5; dy++ {
			for dx := -5; dx <= 5; dx++ {
				if dx*dx+dy*dy <= 25 {
					imgNightCenter.Set(505+dx, 505+dy, brightColor)
				}
			}
		}
		res = AnalyzeFrame(imgNightCenter, cfg)
		if !res.BlueLightDetected {
			t.Errorf("nighttime: expected detection in center region, but got false")
		}
	})

	t.Run("nighttime output format and threshold checks", func(t *testing.T) {
		brightColor := color.Gray{Y: 255}
		img := createTestImage(1000, 800, color.Black)
		for dy := -5; dy <= 5; dy++ {
			for dx := -5; dx <= 5; dx++ {
				if dx*dx+dy*dy <= 25 {
					img.Set(505+dx, 505+dy, brightColor)
				}
			}
		}

		// 1. With a low confidence threshold (e.g. 50), it should be detected as positive
		cfgLow := AnalysisConfig{
			DayColorThreshold:        10,
			NightLuminanceThreshold:  180,
			NightBlobMinSize:         80,
			NightBlobMaxSize:         1000,
			NightConfidenceThreshold: 50,
			EnableNightMode:          true,
		}
		resLow := AnalyzeFrame(img, cfgLow)
		if !resLow.BlueLightDetected {
			t.Errorf("expected BlueLightDetected to be true for threshold 50")
		}
		if resLow.Status != "positive" {
			t.Errorf("expected Status to be 'positive', got %q", resLow.Status)
		}
		if resLow.ConfidenceScore < 50 {
			t.Errorf("expected ConfidenceScore >= 50, got %d", resLow.ConfidenceScore)
		}
		if resLow.Justification == "" {
			t.Errorf("expected non-empty Justification")
		}

		// 2. With an extremely high confidence threshold (e.g. 101), it should be negative
		cfgHigh := AnalysisConfig{
			DayColorThreshold:        10,
			NightLuminanceThreshold:  180,
			NightBlobMinSize:         80,
			NightBlobMaxSize:         1000,
			NightConfidenceThreshold: 101,
			EnableNightMode:          true,
		}
		resHigh := AnalyzeFrame(img, cfgHigh)
		if resHigh.BlueLightDetected {
			t.Errorf("expected BlueLightDetected to be false for threshold 101")
		}
		if resHigh.Status != "negative" {
			t.Errorf("expected Status to be 'negative', got %q", resHigh.Status)
		}
	})

	t.Run("nighttime zero confidence candidate rejection", func(t *testing.T) {
		grayColor := color.Gray{Y: 190}
		img := createTestImage(1000, 800, color.Black)
		for dy := -5; dy <= 5; dy++ {
			for dx := -5; dx <= 5; dx++ {
				if dx*dx+dy*dy <= 25 {
					img.Set(505+dx, 505+dy, grayColor)
				}
			}
		}

		cfgTest := AnalysisConfig{
			DayColorThreshold:        10,
			NightLuminanceThreshold:  180,
			NightBlobMinSize:         80,
			NightBlobMaxSize:         1000,
			NightConfidenceThreshold: 50,
			EnableNightMode:          true,
		}

		res := AnalyzeFrame(img, cfgTest)
		if res.BlueLightDetected {
			t.Errorf("expected BlueLightDetected to be false for 0-confidence blob")
		}
		if res.ConfidenceScore != 0 {
			t.Errorf("expected ConfidenceScore to be 0, got %d", res.ConfidenceScore)
		}
		if res.BluePixelCount != 0 {
			t.Errorf("expected BluePixelCount to be 0, got %d", res.BluePixelCount)
		}
		expectedJustification := "No valid bright blob matching LED criteria was detected"
		if res.Justification != expectedJustification {
			t.Errorf("expected Justification to be %q, got %q", expectedJustification, res.Justification)
		}
	})

	t.Run("custom ROI filtering", func(t *testing.T) {
		cfgCustom := AnalysisConfig{
			DayColorThreshold:        10,
			NightLuminanceThreshold:  180,
			NightBlobMinSize:         80,
			NightBlobMaxSize:         1000,
			NightConfidenceThreshold: 80,
			EnableNightMode:          true,
			ROIXMin:                  0.2,
			ROIXMax:                  0.4,
			ROIYMin:                  0.3,
			ROIYMax:                  0.5,
		}

		blueColor := color.RGBA{R: 50, G: 50, B: 240, A: 255}
		redBackground := color.RGBA{R: 200, G: 0, B: 0, A: 255}

		// 1. Blob center is outside the custom ROI: X center = 1000 (0.5), Y center = 800 (0.4)
		imgOutside := createTestImage(2000, 2000, redBackground)
		for y := 790; y < 810; y++ {
			for x := 990; x < 1010; x++ {
				imgOutside.Set(x, y, blueColor)
			}
		}
		resOutside := AnalyzeFrame(imgOutside, cfgCustom)
		if resOutside.BlueLightDetected {
			t.Errorf("expected no detection since blob is outside custom ROI")
		}

		// 2. Blob center is inside the custom ROI: X center = 600 (0.3), Y center = 800 (0.4)
		imgInside := createTestImage(2000, 2000, redBackground)
		for y := 790; y < 810; y++ {
			for x := 590; x < 610; x++ {
				imgInside.Set(x, y, blueColor)
			}
		}
		resInside := AnalyzeFrame(imgInside, cfgCustom)
		if !resInside.BlueLightDetected {
			t.Errorf("expected detection since blob is inside custom ROI")
		}
	})

	t.Run("invalid ROI fallback", func(t *testing.T) {
		cfgInvalid := AnalysisConfig{
			DayColorThreshold:        10,
			NightLuminanceThreshold:  180,
			NightBlobMinSize:         80,
			NightBlobMaxSize:         1000,
			NightConfidenceThreshold: 80,
			EnableNightMode:          true,
			ROIXMin:                  -0.5, // Invalid value (outside [0.0, 1.0])
			ROIXMax:                  0.4,
			ROIYMin:                  0.3,
			ROIYMax:                  0.5,
		}

		blueColor := color.RGBA{R: 50, G: 50, B: 240, A: 255}
		redBackground := color.RGBA{R: 200, G: 0, B: 0, A: 255}

		// Since cfgInvalid coordinates are invalid, it should fall back to defaults (X in [0.62, 0.72], Y in [0.76, 0.84]).
		// Let's place a blob in the default ROI: center X = 1380 (0.69), center Y = 1600 (0.80) on 2000x2000 image.
		imgFallback := createTestImage(2000, 2000, redBackground)
		for y := 1590; y < 1610; y++ {
			for x := 1370; x < 1390; x++ {
				imgFallback.Set(x, y, blueColor)
			}
		}
		res := AnalyzeFrame(imgFallback, cfgInvalid)
		if !res.BlueLightDetected {
			t.Errorf("expected detection since invalid ROI coordinates should fall back to default ROI bounds")
		}
	})
}

func TestCameraSnapshotsIntegration(t *testing.T) {
	cfg := AnalysisConfig{
		DayColorThreshold:        50,
		NightLuminanceThreshold:  180,
		NightBlobMinSize:         80,
		NightBlobMaxSize:         1000,
		NightConfidenceThreshold: 80,
		EnableNightMode:          true,
		ROIXMin:                  0.62,
		ROIXMax:                  0.72,
		ROIYMin:                  0.76,
		ROIYMax:                  0.84,
	}

	t.Run("daytime-negatives", func(t *testing.T) {
		files, err := filepath.Glob("images/daytime-negative/*.jpg")
		if err != nil {
			t.Fatal(err)
		}
		if len(files) == 0 {
			t.Log("Warning: No negative snapshots found to test")
			return
		}
		for _, file := range files {
			f, err := os.Open(file)
			if err != nil {
				t.Errorf("failed to open %s: %v", file, err)
				continue
			}
			img, _, err := image.Decode(f)
			_ = f.Close()
			if err != nil {
				t.Errorf("failed to decode %s: %v", file, err)
				continue
			}
			res := AnalyzeFrame(img, cfg)
			if res.BlueLightDetected {
				t.Errorf("expected file %s to be negative, but got positive (blue light: %d/%d px)", file, res.BluePixelCount, cfg.DayColorThreshold)
			}
		}
	})

	t.Run("daytime-positives", func(t *testing.T) {
		files, err := filepath.Glob("images/daytime-positive/*.jpg")
		if err != nil {
			t.Fatal(err)
		}
		if len(files) == 0 {
			t.Log("Warning: No blue light snapshots found to test")
			return
		}
		for _, file := range files {
			f, err := os.Open(file)
			if err != nil {
				t.Errorf("failed to open %s: %v", file, err)
				continue
			}
			img, _, err := image.Decode(f)
			_ = f.Close()
			if err != nil {
				t.Errorf("failed to decode %s: %v", file, err)
				continue
			}
			res := AnalyzeFrame(img, cfg)
			if !res.BlueLightDetected {
				t.Errorf("expected file %s to be positive, but got negative (blue light: %d/%d px)", file, res.BluePixelCount, cfg.DayColorThreshold)
			}
		}
	})

	t.Run("night-negatives", func(t *testing.T) {
		files, err := filepath.Glob("images/night-negative/*.jpg")
		if err != nil {
			t.Fatal(err)
		}
		if len(files) == 0 {
			t.Log("Warning: No night negative snapshots found to test")
			return
		}
		for _, file := range files {
			f, err := os.Open(file)
			if err != nil {
				t.Errorf("failed to open %s: %v", file, err)
				continue
			}
			img, _, err := image.Decode(f)
			_ = f.Close()
			if err != nil {
				t.Errorf("failed to decode %s: %v", file, err)
				continue
			}
			res := AnalyzeFrame(img, cfg)
			if res.BlueLightDetected {
				t.Errorf("expected night negative file %s to be negative, but got positive (blue light: %d/%d px)", file, res.BluePixelCount, cfg.NightBlobMinSize)
			}
		}
	})

	t.Run("night-positives", func(t *testing.T) {
		files, err := filepath.Glob("images/night-positive/*.jpg")
		if err != nil {
			t.Fatal(err)
		}
		if len(files) == 0 {
			t.Log("Warning: No night blue light snapshots found to test")
			return
		}
		for _, file := range files {
			f, err := os.Open(file)
			if err != nil {
				t.Errorf("failed to open %s: %v", file, err)
				continue
			}
			img, _, err := image.Decode(f)
			_ = f.Close()
			if err != nil {
				t.Errorf("failed to decode %s: %v", file, err)
				continue
			}
			res := AnalyzeFrame(img, cfg)
			if !res.BlueLightDetected {
				t.Errorf("expected night positive file %s to be positive, but got negative (blue light: %d/%d px)", file, res.BluePixelCount, cfg.NightBlobMinSize)
			}
		}
	})

	t.Run("night-positives with EnableNightMode=false", func(t *testing.T) {
		cfgDisabled := AnalysisConfig{
			DayColorThreshold:        50,
			NightLuminanceThreshold:  180,
			NightBlobMinSize:         80,
			NightBlobMaxSize:         1000,
			NightConfidenceThreshold: 80,
			EnableNightMode:          false,
			ROIXMin:                  0.62,
			ROIXMax:                  0.72,
			ROIYMin:                  0.76,
			ROIYMax:                  0.84,
		}

		files, err := filepath.Glob("images/night-positive/*.jpg")
		if err != nil {
			t.Fatal(err)
		}
		if len(files) == 0 {
			t.Log("Warning: No night blue light snapshots found to test")
			return
		}
		for _, file := range files {
			f, err := os.Open(file)
			if err != nil {
				t.Errorf("failed to open %s: %v", file, err)
				continue
			}
			img, _, err := image.Decode(f)
			_ = f.Close()
			if err != nil {
				t.Errorf("failed to decode %s: %v", file, err)
				continue
			}
			res := AnalyzeFrame(img, cfgDisabled)
			if res.BlueLightDetected {
				t.Errorf("expected night positive file %s to be classified negative because EnableNightMode is false, but got positive", file)
			}
		}
	})
}

func TestLoadAppConfig(t *testing.T) {
	// Set environment variables to test standalone fallback
	t.Setenv("RTSP_URI", "rtsp://localhost/test")
	t.Setenv("DAY_COLOR_THRESHOLD", "120")
	t.Setenv("NIGHT_LUMINANCE_THRESHOLD", "190")
	t.Setenv("NIGHT_BLOB_MIN_SIZE", "90")
	t.Setenv("NIGHT_BLOB_MAX_SIZE", "450")
	t.Setenv("NIGHT_CONFIDENCE_THRESHOLD", "85")
	t.Setenv("ENABLE_NIGHT_MODE", "false")
	t.Setenv("DEBUG_MODE", "true")
	t.Setenv("MQTT_BROKER", "tcp://192.168.1.50:1883")
	t.Setenv("SENSOR_PIN", "22")
	t.Setenv("ROI_X_MIN", "0.1")
	t.Setenv("ROI_X_MAX", "0.2")
	t.Setenv("ROI_Y_MIN", "0.3")
	t.Setenv("ROI_Y_MAX", "0.4")

	cfg, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("failed to load app config: %v", err)
	}

	if cfg.RTSPURI != "rtsp://localhost/test" {
		t.Errorf("expected RTSPURI to be rtsp://localhost/test, got %s", cfg.RTSPURI)
	}
	if cfg.DayColorThreshold != 120 {
		t.Errorf("expected DayColorThreshold to be 120, got %d", cfg.DayColorThreshold)
	}
	if cfg.NightLuminanceThreshold != 190 {
		t.Errorf("expected NightLuminanceThreshold to be 190, got %d", cfg.NightLuminanceThreshold)
	}
	if cfg.NightBlobMinSize != 90 {
		t.Errorf("expected NightBlobMinSize to be 90, got %d", cfg.NightBlobMinSize)
	}
	if cfg.NightBlobMaxSize != 450 {
		t.Errorf("expected NightBlobMaxSize to be 450, got %d", cfg.NightBlobMaxSize)
	}
	if cfg.NightConfidenceThreshold != 85 {
		t.Errorf("expected NightConfidenceThreshold to be 85, got %d", cfg.NightConfidenceThreshold)
	}
	if cfg.EnableNightMode {
		t.Errorf("expected EnableNightMode to be false, got true")
	}
	if !cfg.DebugMode {
		t.Errorf("expected DebugMode to be true")
	}
	if cfg.MQTTBroker != "tcp://192.168.1.50:1883" {
		t.Errorf("expected MQTTBroker to be tcp://192.168.1.50:1883, got %s", cfg.MQTTBroker)
	}
	if cfg.SensorPin != 22 {
		t.Errorf("expected SensorPin to be 22, got %d", cfg.SensorPin)
	}
	if cfg.ROIXMin != 0.1 {
		t.Errorf("expected ROIXMin to be 0.1, got %f", cfg.ROIXMin)
	}
	if cfg.ROIXMax != 0.2 {
		t.Errorf("expected ROIXMax to be 0.2, got %f", cfg.ROIXMax)
	}
	if cfg.ROIYMin != 0.3 {
		t.Errorf("expected ROIYMin to be 0.3, got %f", cfg.ROIYMin)
	}
	if cfg.ROIYMax != 0.4 {
		t.Errorf("expected ROIYMax to be 0.4, got %f", cfg.ROIYMax)
	}

	t.Run("backward compatibility with DETECTION_THRESHOLD", func(t *testing.T) {
		t.Setenv("DETECTION_THRESHOLD", "75")
		// clear DAY_COLOR_THRESHOLD to test fallback
		_ = os.Unsetenv("DAY_COLOR_THRESHOLD")

		cfg, err := LoadAppConfig()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.DayColorThreshold != 75 {
			t.Errorf("expected DayColorThreshold fallback to be 75, got %d", cfg.DayColorThreshold)
		}
	})
}

func TestMQTTPayloads(t *testing.T) {
	// 1. Verify Discovery Payload
	stateTopic := "homeassistant/binary_sensor/kitchen_camera/state"
	attributesTopic := "homeassistant/binary_sensor/kitchen_camera/attributes"
	discPayload := BuildDiscoveryPayload(stateTopic, attributesTopic)

	if discPayload["name"] != "Kitchen Camera Blue Light" {
		t.Errorf("expected name to be 'Kitchen Camera Blue Light', got %v", discPayload["name"])
	}
	if discPayload["unique_id"] != "kitchen_camera_blue_light" {
		t.Errorf("expected unique_id to be 'kitchen_camera_blue_light', got %v", discPayload["unique_id"])
	}
	if discPayload["device_class"] != "light" {
		t.Errorf("expected device_class to be 'light', got %v", discPayload["device_class"])
	}
	if discPayload["state_topic"] != stateTopic {
		t.Errorf("expected state_topic to be %s, got %v", stateTopic, discPayload["state_topic"])
	}
	if discPayload["json_attributes_topic"] != attributesTopic {
		t.Errorf("expected json_attributes_topic to be %s, got %v", attributesTopic, discPayload["json_attributes_topic"])
	}
	if discPayload["payload_on"] != "positive" {
		t.Errorf("expected payload_on to be 'positive', got %v", discPayload["payload_on"])
	}
	if discPayload["payload_off"] != "negative" {
		t.Errorf("expected payload_off to be 'negative', got %v", discPayload["payload_off"])
	}

	// 2. Verify Attributes Struct Marshaling
	attrs := AttributesPayload{
		CurrentMode:           "nighttime",
		NightModeEnabled:      false,
		ConsecutiveStateCount: 2,
		LastDetectionTime:     "2026-06-28T10:00:13Z",
		MatchingPixels:        142,
		AppliedThreshold:      80,
		GrayscaleScore:        5.2,
		ConfidenceScore:       75,
		Status:                "negative",
		Justification:         "LED below threshold",
	}

	bytes, err := json.Marshal(attrs)
	if err != nil {
		t.Fatalf("failed to marshal attributes payload: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(bytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal attributes payload: %v", err)
	}

	if parsed["current_mode"] != attrs.CurrentMode {
		t.Errorf("expected current_mode to be %s, got %v", attrs.CurrentMode, parsed["current_mode"])
	}
	if parsed["night_mode_enabled"] != attrs.NightModeEnabled {
		t.Errorf("expected night_mode_enabled to be %t, got %v", attrs.NightModeEnabled, parsed["night_mode_enabled"])
	}
	if parsed["consecutive_state_count"] != float64(attrs.ConsecutiveStateCount) {
		t.Errorf("expected consecutive_state_count to be %d, got %v", attrs.ConsecutiveStateCount, parsed["consecutive_state_count"])
	}
	if parsed["last_detection_time"] != attrs.LastDetectionTime {
		t.Errorf("expected last_detection_time to be %s, got %v", attrs.LastDetectionTime, parsed["last_detection_time"])
	}
	if parsed["matching_pixels"] != float64(attrs.MatchingPixels) {
		t.Errorf("expected matching_pixels to be %d, got %v", attrs.MatchingPixels, parsed["matching_pixels"])
	}
	if parsed["applied_threshold"] != float64(attrs.AppliedThreshold) {
		t.Errorf("expected applied_threshold to be %d, got %v", attrs.AppliedThreshold, parsed["applied_threshold"])
	}
	if parsed["gray_variance"] != attrs.GrayscaleScore {
		t.Errorf("expected gray_variance to be %f, got %v", attrs.GrayscaleScore, parsed["gray_variance"])
	}
	if parsed["confidence_score"] != float64(attrs.ConfidenceScore) {
		t.Errorf("expected confidence_score to be %d, got %v", attrs.ConfidenceScore, parsed["confidence_score"])
	}
	if parsed["status"] != attrs.Status {
		t.Errorf("expected status to be %s, got %v", attrs.Status, parsed["status"])
	}
	if parsed["justification"] != attrs.Justification {
		t.Errorf("expected justification to be %s, got %v", attrs.Justification, parsed["justification"])
	}
}

func TestStateStabilization(t *testing.T) {
	debouncer := NewStateDebouncer()

	// Initial state is negative
	if debouncer.CurrentOfficialState != "negative" {
		t.Fatalf("expected initial state to be negative, got %s", debouncer.CurrentOfficialState)
	}

	type step struct {
		raw              string
		expectedOfficial string
		expectedCount    int
		expectedChange   bool
	}

	steps := []step{
		// 1. First raw positive: no transition, count=1
		{raw: "positive", expectedOfficial: "negative", expectedCount: 1, expectedChange: false},
		// 2. Flap back to negative: count reset to 0 (since raw matches current official state)
		{raw: "negative", expectedOfficial: "negative", expectedCount: 0, expectedChange: false},
		// 3. Raw positive: count=1
		{raw: "positive", expectedOfficial: "negative", expectedCount: 1, expectedChange: false},
		// 4. Second raw positive: count=2
		{raw: "positive", expectedOfficial: "negative", expectedCount: 2, expectedChange: false},
		// 5. Flap to negative: count=0 (raw matches current official state)
		{raw: "negative", expectedOfficial: "negative", expectedCount: 0, expectedChange: false},
		// 6. First raw positive again: count=1
		{raw: "positive", expectedOfficial: "negative", expectedCount: 1, expectedChange: false},
		// 7. Second raw positive: count=2
		{raw: "positive", expectedOfficial: "negative", expectedCount: 2, expectedChange: false},
		// 8. Third raw positive: transitions to positive, count=0
		{raw: "positive", expectedOfficial: "positive", expectedCount: 0, expectedChange: true},

		// 9. Stable positive: count=0
		{raw: "positive", expectedOfficial: "positive", expectedCount: 0, expectedChange: false},

		// 10. First raw negative: count=1
		{raw: "negative", expectedOfficial: "positive", expectedCount: 1, expectedChange: false},
		// 11. Second raw negative: count=2
		{raw: "negative", expectedOfficial: "positive", expectedCount: 2, expectedChange: false},
		// 12. Third raw negative: transitions back to negative, count=0
		{raw: "negative", expectedOfficial: "negative", expectedCount: 0, expectedChange: true},
	}

	for idx, s := range steps {
		official, count, changed := debouncer.Update(s.raw)
		if official != s.expectedOfficial {
			t.Errorf("step %d: expected official state %s, got %s", idx+1, s.expectedOfficial, official)
		}
		if count != s.expectedCount {
			t.Errorf("step %d: expected consecutive count %d, got %d", idx+1, s.expectedCount, count)
		}
		if changed != s.expectedChange {
			t.Errorf("step %d: expected changed flag %t, got %t", idx+1, s.expectedChange, changed)
		}
	}
}

func TestIsInOvenROI(t *testing.T) {
	bounds := image.Rect(0, 0, 2000, 2000)

	// Helper for cleaner tests
	testCase := func(name string, minX, maxX, minY, maxY int, cfg AnalysisConfig, expected bool) {
		t.Run(name, func(t *testing.T) {
			res := isInOvenROI(minX, maxX, minY, maxY, bounds, cfg)
			if res != expected {
				t.Errorf("expected %t, got %t", expected, res)
			}
		})
	}

	// 1. If width is under 2000, it should always pass
	smallBounds := image.Rect(0, 0, 1000, 1000)
	t.Run("width < 2000", func(t *testing.T) {
		cfg := AnalysisConfig{ROIXMin: 0.1, ROIXMax: 0.2, ROIYMin: 0.3, ROIYMax: 0.4}
		res := isInOvenROI(500, 520, 500, 520, smallBounds, cfg)
		if !res {
			t.Errorf("expected true for width < 2000")
		}
	})

	// 2. Valid custom ROI coordinates
	cfgValid := AnalysisConfig{
		ROIXMin: 0.1,
		ROIXMax: 0.3,
		ROIYMin: 0.2,
		ROIYMax: 0.4,
	}
	// Center: X=200 (0.1), Y=600 (0.3). Matches custom ROI.
	testCase("valid custom ROI - inside", 190, 210, 590, 610, cfgValid, true)
	// Center: X=1000 (0.5), Y=600 (0.3). Outside custom ROI.
	testCase("valid custom ROI - outside", 990, 1010, 590, 610, cfgValid, false)

	// 3. Invalid custom ROI (X_MIN < 0.0) -> Fallback to defaults X in [0.62, 0.72], Y in [0.76, 0.84]
	cfgInvalid1 := AnalysisConfig{
		ROIXMin: -0.1,
		ROIXMax: 0.3,
		ROIYMin: 0.2,
		ROIYMax: 0.4,
	}
	// Center: X=1380 (0.69), Y=1600 (0.80). Matches default ROI.
	testCase("invalid ROI (min < 0) - inside fallback", 1370, 1390, 1590, 1610, cfgInvalid1, true)
	// Center: X=200 (0.1), Y=600 (0.3). Matches custom ROI bounds but should fall back and fail.
	testCase("invalid ROI (min < 0) - outside fallback", 190, 210, 590, 610, cfgInvalid1, false)

	// 4. Invalid custom ROI (Y_MAX > 1.0) -> Fallback to defaults
	cfgInvalid2 := AnalysisConfig{
		ROIXMin: 0.1,
		ROIXMax: 0.3,
		ROIYMin: 0.2,
		ROIYMax: 1.5,
	}
	// Center: X=1380 (0.69), Y=1600 (0.80). Matches default ROI.
	testCase("invalid ROI (max > 1) - inside fallback", 1370, 1390, 1590, 1610, cfgInvalid2, true)

	// 5. Invalid custom ROI (Min >= Max) -> Fallback to defaults
	cfgInvalid3 := AnalysisConfig{
		ROIXMin: 0.4,
		ROIXMax: 0.2,
		ROIYMin: 0.2,
		ROIYMax: 0.4,
	}
	// Center: X=1380 (0.69), Y=1600 (0.80). Matches default ROI.
	testCase("invalid ROI (min >= max) - inside fallback", 1370, 1390, 1590, 1610, cfgInvalid3, true)
}
