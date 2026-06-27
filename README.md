# Kitchen Camera: RTSP Visual Trigger CLI

A production-ready, cross-platform CLI tool written in Go that ingests an RTSP video stream, periodically captures keyframes, and analyzes them for specific visual triggers.

## Features
- **RTSP Stream Ingestion:** Native protocol connection using `gortsplib/v5` with built-in H.264 depacketization.
- **Subprocess Image Decoding:** Pipes raw H.264 keyframes to an external FFmpeg subprocess for standard decoding to JPEG, keeping the Go binary CGO-free, lightweight, and portable.
- **Visual Trigger Detection:** Pure-Go color thresholding and pixel cluster verification logic to check for the presence of a **blue light source** (detects highly saturated, bright blue lights such as indicator beacons).
- **Graceful Shutdown & Reconnection:** Automatically reconnects with backoff if the stream drops, and shuts down cleanly on OS interrupts (`SIGINT`, `SIGTERM`).
- **Native FreeBSD & Linux Support:** Fully containerized multi-stage builds compatible with Linux Docker and native FreeBSD Podman.

## Setup & Configuration

Configure the application using the following environment variables:

| Variable | Description | Default |
| :--- | :--- | :--- |
| `RTSP_URI` | **Required.** The URI of the target RTSP stream (e.g. `rtsp://user:pass@192.168.1.100:554/stream`). | *None* |
| `DETECTION_THRESHOLD` | Optional. The minimum number of pixels in a frame matching the trigger condition to count as `positive`. | `50` |
| `DEBUG_MODE` | Optional. When set to `true`, enables verbose logging showing pixel counts, saves positive frames to the `images/` directory, and outputs detailed MQTT status and payload logs. | `false` |
| `MQTT_BROKER` | Optional. The MQTT broker URI (e.g. `tcp://192.168.1.100:1883`). If omitted, the MQTT module is disabled. | *None* |
| `MQTT_CLIENT_ID` | Optional. Client identifier for the MQTT connection. | `kitchen-camera-cli` |
| `MQTT_USER` | Optional. Username for MQTT authentication. | *None* |
| `MQTT_PASSWORD` | Optional. Password for MQTT authentication. | *None* |
| `MQTT_TOPIC_PREFIX` | Optional. Base topic for Home Assistant auto-discovery config and state topics. | `homeassistant` |

## Usage & Build Commands

Build automation is managed using `GNU Make`. You can override the Go compiler binary by passing the `GO` variable (e.g. `make GO=go1.26 build` or `GO=go1.26 make test`).

### 1. Run Automated Unit Tests
Verify trigger classification algorithms locally using in-memory mock frames:
```bash
make test
```

### 2. Build Local Binary
Compiles the Go binary on your current system:
```bash
make build
```

### 3. Build Container Image
Detects your host OS and container manager (Docker or Podman) to build the correct OCI-compliant container:
```bash
make docker-build
```
- **On Linux:** Uses Docker to build a minimal image based on `alpine:latest`.
- **On FreeBSD:** Uses Podman to build a native FreeBSD image based on `freebsd:latest`.

### 4. Running the Container
Run the built container image by passing the required environment variables:

For Linux (Docker):
```bash
docker run -e RTSP_URI="rtsp://your-stream-url" -e DETECTION_THRESHOLD="50" kitchen-camera:latest
```

For FreeBSD (Podman):
```bash
podman run -e RTSP_URI="rtsp://your-stream-url" -e DETECTION_THRESHOLD="50" kitchen-camera:latest
```
