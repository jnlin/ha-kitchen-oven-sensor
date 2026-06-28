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
		DayColorThreshold:       10,
		NightLuminanceThreshold: 180,
		NightBlobMinSize:        80,
		NightBlobMaxSize:        400,
		EnableNightMode:         true,
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
}

func TestCameraSnapshotsIntegration(t *testing.T) {
	cfg := AnalysisConfig{
		DayColorThreshold:       50,
		NightLuminanceThreshold: 180,
		NightBlobMinSize:        80,
		NightBlobMaxSize:        400,
		EnableNightMode:         true,
	}

	t.Run("negatives", func(t *testing.T) {
		files, err := filepath.Glob("images/negative/*.jpg")
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

	t.Run("positives", func(t *testing.T) {
		files, err := filepath.Glob("images/bluelight/*.jpg")
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
		files, err := filepath.Glob("images/night-bluelight/*.jpg")
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
			DayColorThreshold:       50,
			NightLuminanceThreshold: 180,
			NightBlobMinSize:        80,
			NightBlobMaxSize:        400,
			EnableNightMode:         false,
		}

		files, err := filepath.Glob("images/night-bluelight/*.jpg")
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
	t.Setenv("ENABLE_NIGHT_MODE", "false")
	t.Setenv("DEBUG_MODE", "true")
	t.Setenv("MQTT_BROKER", "tcp://192.168.1.50:1883")
	t.Setenv("SENSOR_PIN", "22")

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
