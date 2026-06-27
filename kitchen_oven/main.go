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

	log.Printf("Starting RTSP Frame Processor (Interval: 10s, Threshold: %d pixels, Debug: %t, Sensor Pin: %d)", cfg.DetectionThreshold, cfg.DebugMode, cfg.SensorPin)

	// Start background analyzer worker
	go analyzerWorker(ctx, frameChan, cfg.DetectionThreshold, cfg.DebugMode, mqttMgr)

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

func analyzerWorker(ctx context.Context, frameChan <-chan FrameData, threshold int, debugMode bool, mqttMgr *MQTTManager) {
	ticker := time.NewTicker(defaultInterval)
	defer ticker.Stop()

	var lastFrame *FrameData

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

			res := AnalyzeFrame(img, threshold)
			isPositive := res.BlueLightDetected

			var resultStr string
			if isPositive {
				resultStr = "positive"
				if debugMode {
					if err := savePositiveImage(img); err != nil {
						fmt.Fprintf(os.Stderr, "Error saving positive image: %v\n", err)
					}
				}
			} else {
				resultStr = "negative"
			}

			// Publish to MQTT if active
			if mqttMgr != nil {
				mqttMgr.PublishState(resultStr)
			}

			// Output result with details in debug mode
			if debugMode {
				// Show why it is positive or negative, which condition was fulfilled
				var reason string
				if res.BlueLightDetected {
					reason = fmt.Sprintf("blue light condition met: blue light (%d/%d px)", res.BluePixelCount, threshold)
				} else {
					reason = fmt.Sprintf("blue light condition not met: blue light (%d/%d px)", res.BluePixelCount, threshold)
				}
				fmt.Printf("%s (%s)\n", resultStr, reason)
			} else {
				fmt.Println(resultStr)
			}
		}
	}
}

func savePositiveImage(img image.Image) error {
	dir := "images"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(dir, fmt.Sprintf("snapshot_%s.jpg", timestamp))

	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create image file: %w", err)
	}
	defer f.Close()

	err = jpeg.Encode(f, img, &jpeg.Options{Quality: 90})
	if err != nil {
		return fmt.Errorf("failed to encode jpeg: %w", err)
	}

	log.Printf("Saved positive frame to %s", filename)
	return nil
}
