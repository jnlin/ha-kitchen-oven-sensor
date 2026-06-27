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
	threshold := 10

	t.Run("both absent (black background)", func(t *testing.T) {
		img := createTestImage(100, 100, color.Black)
		res := AnalyzeFrame(img, threshold)
		if res.BlueLightDetected {
			t.Errorf("expected no blue light, but got blue light detected")
		}
	})

	t.Run("fire present (should be negative)", func(t *testing.T) {
		img := createTestImage(100, 100, color.Black)
		// Set pixels matching fire color (which is NOT blue light)
		fireColor := color.RGBA{R: 240, G: 150, B: 50, A: 255}
		for i := 0; i < threshold; i++ {
			img.Set(i, 0, fireColor)
		}
		res := AnalyzeFrame(img, threshold)
		if res.BlueLightDetected {
			t.Errorf("expected no blue light, but got blue light detected")
		}
	})

	t.Run("blue light present", func(t *testing.T) {
		img := createTestImage(100, 100, color.Black)
		// Set pixels matching blue light condition: B > 180, B > R+80, B > G+80
		blueColor := color.RGBA{R: 50, G: 50, B: 240, A: 255}
		for i := 0; i < threshold; i++ {
			img.Set(i, 0, blueColor)
		}
		res := AnalyzeFrame(img, threshold)
		if !res.BlueLightDetected {
			t.Errorf("expected blue light detected, but got false")
		}
	})

	t.Run("both present (should detect blue light)", func(t *testing.T) {
		img := createTestImage(100, 100, color.Black)
		fireColor := color.RGBA{R: 240, G: 150, B: 50, A: 255}
		blueColor := color.RGBA{R: 50, G: 50, B: 240, A: 255}
		for i := 0; i < threshold; i++ {
			img.Set(i, 0, fireColor)
			img.Set(i, 1, blueColor)
		}
		res := AnalyzeFrame(img, threshold)
		if !res.BlueLightDetected {
			t.Errorf("expected blue light detected, but got false")
		}
	})
}

func TestCameraSnapshotsIntegration(t *testing.T) {
	threshold := 50 // default threshold

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
			res := AnalyzeFrame(img, threshold)
			if res.BlueLightDetected {
				t.Errorf("expected file %s to be negative, but got positive (blue light: %d/%d px)", file, res.BluePixelCount, threshold)
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
			res := AnalyzeFrame(img, threshold)
			if !res.BlueLightDetected {
				t.Errorf("expected file %s to be positive, but got negative (blue light: %d/%d px)", file, res.BluePixelCount, threshold)
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
			res := AnalyzeFrame(img, threshold)
			if res.BlueLightDetected {
				t.Errorf("expected night negative file %s to be negative, but got positive (blue light: %d/%d px)", file, res.BluePixelCount, threshold)
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
			res := AnalyzeFrame(img, threshold)
			if !res.BlueLightDetected {
				t.Errorf("expected night positive file %s to be positive, but got negative (blue light: %d/%d px)", file, res.BluePixelCount, threshold)
			}
		}
	})
}

func TestLoadAppConfig(t *testing.T) {
	// Set environment variables to test standalone fallback
	t.Setenv("RTSP_URI", "rtsp://localhost/test")
	t.Setenv("DETECTION_THRESHOLD", "100")
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
	if cfg.DetectionThreshold != 100 {
		t.Errorf("expected DetectionThreshold to be 100, got %d", cfg.DetectionThreshold)
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
}
