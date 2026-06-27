package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// AppConfig holds the generic configuration values used across the application.
type AppConfig struct {
	RTSPURI            string
	DetectionThreshold int
	DebugMode          bool
	MQTTBroker         string
	MQTTClientID       string
	MQTTUser           string
	MQTTPassword       string
	MQTTTopicPrefix    string
	SensorPin          int
}

// LoadAppConfig resolves the application configuration:
// 1. Attempts to read /data/options.json if it exists (Home Assistant Add-on Mode).
// 2. Otherwise falls back to environment variables (Standalone Mode).
func LoadAppConfig() (*AppConfig, error) {
	const hassOptionsPath = "/data/options.json"

	cfg := &AppConfig{
		DetectionThreshold: 50,
		SensorPin:          14,
		MQTTTopicPrefix:    "homeassistant",
		MQTTClientID:       "kitchen-camera-cli",
	}

	// 1. Home Assistant Mode
	if _, err := os.Stat(hassOptionsPath); err == nil {
		data, err := os.ReadFile(hassOptionsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read HA options file: %w", err)
		}

		var opts struct {
			RTSPURI            string `json:"RTSP_URI"`
			DetectionThreshold int    `json:"DETECTION_THRESHOLD"`
			DebugMode          bool   `json:"DEBUG_MODE"`
			MQTTHost           string `json:"mqtt_host"`
			MQTTPort           int    `json:"mqtt_port"`
			MQTTUser           string `json:"mqtt_user"`
			MQTTPass           string `json:"mqtt_pass"`
			SensorPin          int    `json:"sensor_pin"`
			MQTTTopicPrefix    string `json:"MQTT_TOPIC_PREFIX"`
			MQTTClientID       string `json:"MQTT_CLIENT_ID"`
		}

		if err := json.Unmarshal(data, &opts); err != nil {
			return nil, fmt.Errorf("failed to parse HA options JSON: %w", err)
		}

		cfg.RTSPURI = opts.RTSPURI
		if opts.DetectionThreshold > 0 {
			cfg.DetectionThreshold = opts.DetectionThreshold
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

	if threshStr := os.Getenv("DETECTION_THRESHOLD"); threshStr != "" {
		if val, err := strconv.Atoi(threshStr); err == nil && val > 0 {
			cfg.DetectionThreshold = val
		}
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

	return cfg, nil
}
