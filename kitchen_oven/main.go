package main

import (
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/bluenviron/gortsplib/v5"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/pion/rtp"
)

const (
	defaultInterval  = 10 * time.Second
	defaultThreshold = 50
)

func main() {
	cfg, err := LoadAppConfig()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	if cfg.RTSPURI == "" {
		log.Println("Error: RTSP_URI is not set")
		os.Exit(1)
	}

	// Setup context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down gracefully...", sig)
		cancel()
	}()

	// Conditional MQTT Setup
	var mqttMgr *MQTTManager
	if cfg.MQTTBroker != "" {
		mqttMgr, err = NewMQTTManager(cfg.MQTTBroker, cfg.MQTTClientID, cfg.MQTTUser, cfg.MQTTPassword, cfg.MQTTTopicPrefix, cfg.DebugMode)
		if err != nil {
			log.Fatalf("Failed to initialize MQTT client: %v", err)
		}
		defer mqttMgr.Close()
	}

	// Channel to pass raw frame data from RTSP callback to the analyzer worker
	frameChan := make(chan FrameData, 1)

	analysisCfg := AnalysisConfig{
		DayColorThreshold:       cfg.DayColorThreshold,
		NightLuminanceThreshold: cfg.NightLuminanceThreshold,
		NightBlobMinSize:        cfg.NightBlobMinSize,
		NightBlobMaxSize:        cfg.NightBlobMaxSize,
		EnableNightMode:         cfg.EnableNightMode,
	}

	log.Printf("Starting RTSP Frame Processor (Interval: 10s, DayThreshold: %d, NightLuminanceThreshold: %d, NightBlobMinSize: %d, NightBlobMaxSize: %d, EnableNightMode: %t, Debug: %t, Sensor Pin: %d)", cfg.DayColorThreshold, cfg.NightLuminanceThreshold, cfg.NightBlobMinSize, cfg.NightBlobMaxSize, cfg.EnableNightMode, cfg.DebugMode, cfg.SensorPin)

	// Start background analyzer worker
	go analyzerWorker(ctx, frameChan, analysisCfg, cfg.DebugMode, mqttMgr)

	// Start RTSP client reconnection loop
	runRTSPClient(ctx, cfg.RTSPURI, frameChan)
}

func runRTSPClient(ctx context.Context, rtspURI string, frameChan chan FrameData) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		log.Printf("Connecting to RTSP stream: %s", rtspURI)
		err := connectAndPlay(ctx, rtspURI, frameChan)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			fmt.Fprintf(os.Stderr, "RTSP error: %v. Reconnecting in 5 seconds...\n", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func connectAndPlay(ctx context.Context, rtspURI string, frameChan chan FrameData) error {
	u, err := base.ParseURL(rtspURI)
	if err != nil {
		return fmt.Errorf("invalid RTSP URI: %w", err)
	}

	c := &gortsplib.Client{
		Scheme: u.Scheme,
		Host:   u.Host,
	}
	err = c.Start()
	if err != nil {
		return fmt.Errorf("failed to start client: %w", err)
	}
	defer c.Close()

	desc, _, err := c.Describe(u)
	if err != nil {
		return fmt.Errorf("failed to describe stream: %w", err)
	}

	var h264Media *description.Media
	var h264Form *format.H264
	for _, media := range desc.Medias {
		for _, form := range media.Formats {
			if h264F, ok := form.(*format.H264); ok {
				h264Media = media
				h264Form = h264F
				break
			}
		}
	}

	if h264Media == nil {
		return fmt.Errorf("H264 video track not found in RTSP stream")
	}

	rtpDec, err := h264Form.CreateDecoder()
	if err != nil {
		return fmt.Errorf("failed to create RTP decoder: %w", err)
	}

	_, err = c.Setup(desc.BaseURL, h264Media, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to setup track: %w", err)
	}

	c.OnPacketRTP(h264Media, h264Form, func(pkt *rtp.Packet) {
		nalus, err := rtpDec.Decode(pkt)
		if err != nil {
			return
		}

		isKeyframe := false
		for _, nalu := range nalus {
			if len(nalu) > 0 {
				naluType := nalu[0] & 0x1F
				if naluType == 5 { // IDR frame (keyframe)
					isKeyframe = true
					break
				}
			}
		}

		if isKeyframe {
			// Ensure the channel holds only the *latest* keyframe
			// First attempt to non-blockingly drain the channel
			select {
			case <-frameChan:
			default:
			}
			// Now insert the latest frame
			select {
			case frameChan <- FrameData{
				SPS:   h264Form.SPS,
				PPS:   h264Form.PPS,
				NALUs: nalus,
			}:
			default:
			}
		}
	})

	_, err = c.Play(nil)
	if err != nil {
		return fmt.Errorf("failed to play stream: %w", err)
	}

	waitErr := make(chan error, 1)
	go func() {
		waitErr <- c.Wait()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-waitErr:
		return fmt.Errorf("connection closed: %w", err)
	}
}

// StateDebouncer tracks consecutive frames to stabilize state transitions.
type StateDebouncer struct {
	CurrentOfficialState string
	LastRawState         string
	ConsecutiveCount     int
}

// NewStateDebouncer initializes the debouncer starting at "negative".
func NewStateDebouncer() *StateDebouncer {
	return &StateDebouncer{
		CurrentOfficialState: "negative",
		LastRawState:         "negative",
		ConsecutiveCount:     0,
	}
}

// Update evaluates a raw state and returns (newOfficialState, currentConsecutiveCount, stateChanged).
func (d *StateDebouncer) Update(rawState string) (string, int, bool) {
	stateChanged := false
	transitionCount := 0

	if rawState != d.CurrentOfficialState {
		if rawState == d.LastRawState {
			d.ConsecutiveCount++
		} else {
			d.ConsecutiveCount = 1
		}
		if d.ConsecutiveCount >= 3 {
			d.CurrentOfficialState = rawState
			d.ConsecutiveCount = 0
			stateChanged = true
		}
		transitionCount = d.ConsecutiveCount
	} else {
		d.ConsecutiveCount = 0
		transitionCount = 0
	}
	d.LastRawState = rawState
	return d.CurrentOfficialState, transitionCount, stateChanged
}

func analyzerWorker(ctx context.Context, frameChan <-chan FrameData, analysisCfg AnalysisConfig, debugMode bool, mqttMgr *MQTTManager) {
	ticker := time.NewTicker(defaultInterval)
	defer ticker.Stop()

	var lastFrame *FrameData
	debouncer := NewStateDebouncer()

	for {
		select {
		case <-ctx.Done():
			log.Println("Analyzer worker stopped.")
			return
		case fd := <-frameChan:
			// Store the latest keyframe received
			lastFrame = &fd
		case <-ticker.C:
			if lastFrame == nil {
				log.Println("Waiting for RTSP stream keyframe...")
				continue
			}

			// Capture the frame details and decode with timeout
			decodeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
			img, err := DecodeH264Frame(decodeCtx, *lastFrame)
			cancel()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error decoding frame: %v\n", err)
				continue
			}

			res := AnalyzeFrame(img, analysisCfg)
			rawState := "negative"
			if res.BlueLightDetected {
				rawState = "positive"
			}

			oldOfficialState := debouncer.CurrentOfficialState
			officialState, transitionCount, stateChanged := debouncer.Update(rawState)

			if debugMode {
				if err := saveSnapshotImage(img, rawState); err != nil {
					fmt.Fprintf(os.Stderr, "Error saving snapshot image: %v\n", err)
				}
			}

			// Publish to MQTT if active
			if mqttMgr != nil {
				lastDetectionTime := time.Now().Format(time.RFC3339)
				mqttMgr.PublishState(officialState)
				mqttMgr.PublishAttributes(res.CurrentMode, analysisCfg.EnableNightMode, transitionCount, lastDetectionTime)
			}

			// Output detailed result only in debug mode
			if debugMode {
				var reason string
				if res.BlueLightDetected {
					reason = fmt.Sprintf("condition met: mode=%s, applied_threshold=%d, matching_pixels=%d, gray_variance=%.2f",
						res.CurrentMode, res.AppliedThreshold, res.BluePixelCount, res.GrayscaleScore)
				} else {
					reason = fmt.Sprintf("condition not met: mode=%s, applied_threshold=%d, matching_pixels=%d, gray_variance=%.2f",
						res.CurrentMode, res.AppliedThreshold, res.BluePixelCount, res.GrayscaleScore)
				}
				log.Printf("%s (%s)", rawState, reason)

				if stateChanged {
					log.Printf("[DEBUG] Raw detection: %s. Consecutive count for %s: 3/3. Current official MQTT state transitions to: %s.", rawState, rawState, officialState)
				} else {
					log.Printf("[DEBUG] Raw detection: %s. Consecutive count for %s: %d/3. Current official MQTT state remains: %s.", rawState, rawState, transitionCount, oldOfficialState)
				}
			}
		}
	}
}

func saveSnapshotImage(img image.Image, status string) error {
	dir := "images"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(dir, fmt.Sprintf("snapshot_%s_%s.jpg", status, timestamp))

	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create image file: %w", err)
	}

	err = jpeg.Encode(f, img, &jpeg.Options{Quality: 90})
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("failed to encode jpeg: %w", err)
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("failed to close image file: %w", err)
	}

	log.Printf("Saved %s frame to %s", status, filename)
	return nil
}
