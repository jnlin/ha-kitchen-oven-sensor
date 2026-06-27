package main

import (
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
}

func TestLoadAppConfig(t *testing.T) {
	// Set environment variables to test standalone fallback
	t.Setenv("RTSP_URI", "rtsp://localhost/test")
	t.Setenv("DAY_COLOR_THRESHOLD", "120")
	t.Setenv("NIGHT_LUMINANCE_THRESHOLD", "190")
	t.Setenv("NIGHT_BLOB_MIN_SIZE", "90")
	t.Setenv("NIGHT_BLOB_MAX_SIZE", "450")
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

	// 2. Verify Attributes Payload
	currentMode := "nighttime"
	appliedThreshold := 125
	lastDetectionTime := "2026-06-27T19:12:00Z"
	attrPayload := BuildAttributesPayload(currentMode, appliedThreshold, lastDetectionTime)

	if attrPayload["current_mode"] != currentMode {
		t.Errorf("expected current_mode to be %s, got %v", currentMode, attrPayload["current_mode"])
	}
	if attrPayload["applied_threshold"] != appliedThreshold {
		t.Errorf("expected applied_threshold to be %d, got %v", appliedThreshold, attrPayload["applied_threshold"])
	}
	if attrPayload["last_detection_time"] != lastDetectionTime {
		t.Errorf("expected last_detection_time to be %s, got %v", lastDetectionTime, attrPayload["last_detection_time"])
	}
}
