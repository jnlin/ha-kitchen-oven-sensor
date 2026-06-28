package main

import (
	"encoding/json"
	"fmt"
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTTManager handles state publication and Home Assistant MQTT discovery registrations.
type MQTTManager struct {
	client          mqtt.Client
	debug           bool
	stateTopic      string
	attributesTopic string
}

// AttributesPayload wraps all the frame operational metadata sent to Home Assistant.
type AttributesPayload struct {
	CurrentMode           string  `json:"current_mode"`
	NightModeEnabled      bool    `json:"night_mode_enabled"`
	ConsecutiveStateCount int     `json:"consecutive_state_count"`
	LastDetectionTime     string  `json:"last_detection_time"`
	MatchingPixels        int     `json:"matching_pixels"`
	AppliedThreshold      int     `json:"applied_threshold"`
	GrayscaleScore        float64 `json:"gray_variance"`
}

// BuildDiscoveryPayload builds the HA discovery registration map.
func BuildDiscoveryPayload(stateTopic, attributesTopic string) map[string]interface{} {
	return map[string]interface{}{
		"name":                  "Kitchen Camera Blue Light",
		"state_topic":           stateTopic,
		"json_attributes_topic": attributesTopic,
		"unique_id":             "kitchen_camera_blue_light",
		"device_class":          "light",
		"payload_on":            "positive",
		"payload_off":           "negative",
		"value_template":        "{{ value }}",
	}
}

// NewMQTTManager initializes a new Paho MQTT client and connects to the specified broker.
func NewMQTTManager(broker, clientID, user, password, topicPrefix string, debug bool) (*MQTTManager, error) {
	if clientID == "" {
		clientID = "kitchen-camera-cli"
	}
	if topicPrefix == "" {
		topicPrefix = "homeassistant"
	}

	stateTopic := fmt.Sprintf("%s/binary_sensor/kitchen_camera/state", topicPrefix)
	attributesTopic := fmt.Sprintf("%s/binary_sensor/kitchen_camera/attributes", topicPrefix)
	discoveryTopic := fmt.Sprintf("%s/binary_sensor/kitchen_camera/config", topicPrefix)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	if user != "" {
		opts.SetUsername(user)
	}
	if password != "" {
		opts.SetPassword(password)
	}
	opts.SetAutoReconnect(true)

	// Connection callbacks with required [DEBUG] logging
	opts.OnConnect = func(c mqtt.Client) {
		if debug {
			log.Println("[DEBUG] Successfully connected to MQTT broker")
		}
		// Register Home Assistant discovery config payload
		discoveryPayload := BuildDiscoveryPayload(stateTopic, attributesTopic)
		payloadBytes, err := json.Marshal(discoveryPayload)
		if err != nil {
			log.Printf("Error marshaling discovery payload: %v\n", err)
			return
		}
		if debug {
			log.Printf("[DEBUG] Publishing HA Discovery Payload to topic %s: %s\n", discoveryTopic, string(payloadBytes))
		}
		token := c.Publish(discoveryTopic, 1, true, payloadBytes)
		token.Wait()
		if err := token.Error(); err != nil {
			log.Printf("Error publishing discovery payload: %v\n", err)
		}
	}

	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		if debug {
			log.Printf("[DEBUG] MQTT connection lost: %v\n", err)
		}
	}

	opts.SetReconnectingHandler(func(c mqtt.Client, co *mqtt.ClientOptions) {
		if debug {
			log.Println("[DEBUG] Reconnecting to MQTT broker...")
		}
	})

	if debug {
		log.Println("[DEBUG] Attempting to connect to MQTT broker...")
	}

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	return &MQTTManager{
		client:          client,
		debug:           debug,
		stateTopic:      stateTopic,
		attributesTopic: attributesTopic,
	}, nil
}

// PublishState sends the current state (positive or negative) to the state topic.
func (m *MQTTManager) PublishState(state string) {
	if m.client == nil || !m.client.IsConnected() {
		if m.debug {
			log.Printf("[DEBUG] Cannot publish state; MQTT client is not connected\n")
		}
		return
	}

	if m.debug {
		log.Printf("[DEBUG] Publishing State Payload to topic %s: %s\n", m.stateTopic, state)
	}

	token := m.client.Publish(m.stateTopic, 1, true, state)
	token.Wait()
	if err := token.Error(); err != nil {
		log.Printf("Error publishing state: %v\n", err)
	}
}

// PublishAttributes sends the serialized attributes JSON to the attributes topic.
func (m *MQTTManager) PublishAttributes(attrs AttributesPayload) {
	if m.client == nil || !m.client.IsConnected() {
		if m.debug {
			log.Printf("[DEBUG] Cannot publish attributes; MQTT client is not connected\n")
		}
		return
	}

	payloadBytes, err := json.Marshal(attrs)
	if err != nil {
		log.Printf("Error marshaling attributes payload: %v\n", err)
		return
	}

	if m.debug {
		log.Printf("[DEBUG] Publishing Attributes Payload to topic %s: %s\n", m.attributesTopic, string(payloadBytes))
	}

	token := m.client.Publish(m.attributesTopic, 1, true, payloadBytes)
	token.Wait()
	if err := token.Error(); err != nil {
		log.Printf("Error publishing attributes: %v\n", err)
	}
}

// Close gracefully disconnects the MQTT client.
func (m *MQTTManager) Close() {
	if m.client != nil && m.client.IsConnected() {
		m.client.Disconnect(250)
	}
}
