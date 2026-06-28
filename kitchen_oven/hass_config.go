package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// AppConfig holds the generic configuration values used across the application.
type AppConfig struct {
	RTSPURI                  string
	DayColorThreshold        int
	NightLuminanceThreshold  int
	NightBlobMinSize         int
	NightBlobMaxSize         int
	NightConfidenceThreshold int
	EnableNightMode          bool
	DebugMode                bool
	MQTTBroker               string
	MQTTClientID             string
	MQTTUser                 string
	MQTTPassword             string
	MQTTTopicPrefix          string
	SensorPin                int
	ROIXMin                  float64
	ROIXMax                  float64
	ROIYMin                  float64
	ROIYMax                  float64
}

// LoadAppConfig resolves the application configuration:
// 1. Attempts to read /data/options.json if it exists (Home Assistant Add-on Mode).
// 2. Otherwise falls back to environment variables (Standalone Mode).
func LoadAppConfig() (*AppConfig, error) {
	const hassOptionsPath = "/data/options.json"

	cfg := &AppConfig{
		DayColorThreshold:        50,
		NightLuminanceThreshold:  180,
		NightBlobMinSize:         80,
		NightBlobMaxSize:         1000,
		NightConfidenceThreshold: 80,
		EnableNightMode:          true,
		SensorPin:                14,
		MQTTTopicPrefix:          "homeassistant",
		MQTTClientID:             "kitchen-camera-cli",
		ROIXMin:                  0.62,
		ROIXMax:                  0.72,
		ROIYMin:                  0.76,
		ROIYMax:                  0.84,
	}

	// 1. Home Assistant Mode
	if _, err := os.Stat(hassOptionsPath); err == nil {
		data, err := os.ReadFile(hassOptionsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read HA options file: %w", err)
		}

		var opts struct {
			RTSPURI                  string   `json:"RTSP_URI"`
			DayColorThreshold        int      `json:"DAY_COLOR_THRESHOLD"`
			NightLuminanceThreshold  int      `json:"NIGHT_LUMINANCE_THRESHOLD"`
			NightBlobMinSize         int      `json:"NIGHT_BLOB_MIN_SIZE"`
			NightBlobMaxSize         int      `json:"NIGHT_BLOB_MAX_SIZE"`
			NightConfidenceThreshold int      `json:"NIGHT_CONFIDENCE_THRESHOLD"`
			EnableNightMode          *bool    `json:"ENABLE_NIGHT_MODE"`
			DebugMode                bool     `json:"DEBUG_MODE"`
			MQTTHost                 string   `json:"mqtt_host"`
			MQTTPort                 int      `json:"mqtt_port"`
			MQTTUser                 string   `json:"mqtt_user"`
			MQTTPass                 string   `json:"mqtt_pass"`
			SensorPin                int      `json:"sensor_pin"`
			MQTTTopicPrefix          string   `json:"MQTT_TOPIC_PREFIX"`
			MQTTClientID             string   `json:"MQTT_CLIENT_ID"`
			ROIXMin                  *float64 `json:"ROI_X_MIN"`
			ROIXMax                  *float64 `json:"ROI_X_MAX"`
			ROIYMin                  *float64 `json:"ROI_Y_MIN"`
			ROIYMax                  *float64 `json:"ROI_Y_MAX"`
		}

		if err := json.Unmarshal(data, &opts); err != nil {
			return nil, fmt.Errorf("failed to parse HA options JSON: %w", err)
		}

		cfg.RTSPURI = opts.RTSPURI
		if opts.DayColorThreshold > 0 {
			cfg.DayColorThreshold = opts.DayColorThreshold
		}
		if opts.NightLuminanceThreshold > 0 {
			cfg.NightLuminanceThreshold = opts.NightLuminanceThreshold
		}
		if opts.NightBlobMinSize > 0 {
			cfg.NightBlobMinSize = opts.NightBlobMinSize
		}
		if opts.NightBlobMaxSize > 0 {
			cfg.NightBlobMaxSize = opts.NightBlobMaxSize
		}
		if opts.NightConfidenceThreshold > 0 {
			cfg.NightConfidenceThreshold = opts.NightConfidenceThreshold
		}
		if opts.EnableNightMode != nil {
			cfg.EnableNightMode = *opts.EnableNightMode
		}
		cfg.DebugMode = opts.DebugMode
		cfg.MQTTUser = opts.MQTTUser
		cfg.MQTTPassword = opts.MQTTPass
		if opts.MQTTTopicPrefix != "" {
			cfg.MQTTTopicPrefix = opts.MQTTTopicPrefix
		}
		if opts.MQTTClientID != "" {
			cfg.MQTTClientID = opts.MQTTClientID
		}
		if opts.SensorPin > 0 {
			cfg.SensorPin = opts.SensorPin
		}
		if opts.ROIXMin != nil {
			cfg.ROIXMin = *opts.ROIXMin
		}
		if opts.ROIXMax != nil {
			cfg.ROIXMax = *opts.ROIXMax
		}
		if opts.ROIYMin != nil {
			cfg.ROIYMin = *opts.ROIYMin
		}
		if opts.ROIYMax != nil {
			cfg.ROIYMax = *opts.ROIYMax
		}

		// Formulate broker target from Host and Port
		if opts.MQTTHost != "" {
			port := 1883
			if opts.MQTTPort > 0 {
				port = opts.MQTTPort
			}
			cfg.MQTTBroker = fmt.Sprintf("tcp://%s:%d", opts.MQTTHost, port)
		}
		return cfg, nil
	}

	// 2. Standalone Mode (Fallback to Env Vars)
	cfg.RTSPURI = os.Getenv("RTSP_URI")

	if valStr := os.Getenv("DAY_COLOR_THRESHOLD"); valStr != "" {
		if val, err := strconv.Atoi(valStr); err == nil && val > 0 {
			cfg.DayColorThreshold = val
		}
	} else if valStr := os.Getenv("DETECTION_THRESHOLD"); valStr != "" {
		// Fallback for backward compatibility
		if val, err := strconv.Atoi(valStr); err == nil && val > 0 {
			cfg.DayColorThreshold = val
		}
	}

	if valStr := os.Getenv("NIGHT_LUMINANCE_THRESHOLD"); valStr != "" {
		if val, err := strconv.Atoi(valStr); err == nil && val > 0 {
			cfg.NightLuminanceThreshold = val
		}
	}

	if valStr := os.Getenv("NIGHT_BLOB_MIN_SIZE"); valStr != "" {
		if val, err := strconv.Atoi(valStr); err == nil && val > 0 {
			cfg.NightBlobMinSize = val
		}
	}

	if valStr := os.Getenv("NIGHT_BLOB_MAX_SIZE"); valStr != "" {
		if val, err := strconv.Atoi(valStr); err == nil && val > 0 {
			cfg.NightBlobMaxSize = val
		}
	}

	if valStr := os.Getenv("NIGHT_CONFIDENCE_THRESHOLD"); valStr != "" {
		if val, err := strconv.Atoi(valStr); err == nil && val > 0 {
			cfg.NightConfidenceThreshold = val
		}
	}

	if valStr := os.Getenv("ENABLE_NIGHT_MODE"); valStr == "false" {
		cfg.EnableNightMode = false
	}

	if os.Getenv("DEBUG_MODE") == "true" {
		cfg.DebugMode = true
	}

	// MQTT config: check direct broker URI, otherwise construct from HOST/PORT
	broker := os.Getenv("MQTT_BROKER")
	if broker != "" {
		cfg.MQTTBroker = broker
	} else {
		mqttHost := os.Getenv("MQTT_HOST")
		if mqttHost != "" {
			mqttPort := 1883
			if portStr := os.Getenv("MQTT_PORT"); portStr != "" {
				if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
					mqttPort = p
				}
			}
			cfg.MQTTBroker = fmt.Sprintf("tcp://%s:%d", mqttHost, mqttPort)
		}
	}

	if clientID := os.Getenv("MQTT_CLIENT_ID"); clientID != "" {
		cfg.MQTTClientID = clientID
	}
	cfg.MQTTUser = os.Getenv("MQTT_USER")
	cfg.MQTTPassword = os.Getenv("MQTT_PASSWORD")
	if pass := os.Getenv("MQTT_PASS"); pass != "" {
		cfg.MQTTPassword = pass
	}
	if prefix := os.Getenv("MQTT_TOPIC_PREFIX"); prefix != "" {
		cfg.MQTTTopicPrefix = prefix
	}

	if pinStr := os.Getenv("SENSOR_PIN"); pinStr != "" {
		if val, err := strconv.Atoi(pinStr); err == nil && val > 0 {
			cfg.SensorPin = val
		}
	}

	if valStr := os.Getenv("ROI_X_MIN"); valStr != "" {
		if val, err := strconv.ParseFloat(valStr, 64); err == nil {
			cfg.ROIXMin = val
		}
	}
	if valStr := os.Getenv("ROI_X_MAX"); valStr != "" {
		if val, err := strconv.ParseFloat(valStr, 64); err == nil {
			cfg.ROIXMax = val
		}
	}
	if valStr := os.Getenv("ROI_Y_MIN"); valStr != "" {
		if val, err := strconv.ParseFloat(valStr, 64); err == nil {
			cfg.ROIYMin = val
		}
	}
	if valStr := os.Getenv("ROI_Y_MAX"); valStr != "" {
		if val, err := strconv.ParseFloat(valStr, 64); err == nil {
			cfg.ROIYMax = val
		}
	}

	return cfg, nil
}
