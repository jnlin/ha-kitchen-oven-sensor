# Kitchen Camera: RTSP Visual Trigger CLI

A production-ready, cross-platform CLI tool written in Go that ingests an RTSP video stream, periodically captures keyframes, and analyzes them for specific visual triggers.

## Features
- **RTSP Stream Ingestion:** Native protocol connection using `gortsplib/v5` with built-in H.264 depacketization.
- **Subprocess Image Decoding:** Pipes raw H.264 keyframes to an external FFmpeg subprocess for standard decoding to JPEG, keeping the Go binary CGO-free, lightweight, and portable.
- **Visual Trigger Detection:** Pure-Go color thresholding and pixel cluster verification logic to check for the presence of a **blue light source** (detects highly saturated, bright blue lights such as indicator beacons).
- **Graceful Shutdown & Reconnection:** Automatically reconnects with backoff if the stream drops, and shuts down cleanly on OS interrupts (`SIGINT`, `SIGTERM`).
- **Native FreeBSD & Linux Support:** Fully containerized multi-stage builds compatible with Linux Docker and native FreeBSD Podman.

## Night-Vision (IR) Detection

To handle Infrared (IR) night-vision streams where colors are converted to grayscale and the blue light source manifests as a bright grayscale highlight, the visual trigger logic uses an adaptive mode selector:
1. **Grayscale Detection:** The frame is automatically checked for color saturation by calculating the channel variance ($|R-G| + |R-B| + |G-B|$) over a sampled grid of pixels. If the average variance is $< 10.0$ (on a scale of 0-255), the frame is processed in night-vision mode.
2. **Daytime (Color) Mode:** Scans the entire frame for bright, highly saturated blue pixels matching $B > 180$, $B > R+80$, and $B > G+80$.
3. **Nighttime (IR) Mode (Shift-Invariant Blob Detection):** Instead of using hardcoded coordinates, the algorithm scans the entire image to find connected components (blobs) of bright pixels (intensity $\ge 180$) using a Breadth-First Search (BFS). 

To ensure robustness against camera rotation/pans and filter out date overlays or camera watermarks, blobs are filtered using the following invariants:
* **Border Exclusion:** Blobs centered in the top 400 pixels or bottom 150 pixels of the frame are ignored (safely filtering out time overlays and bottom status indicators).
* **Area Range:** Between $80$ and $400$ pixels.
* **Aspect Ratio:** $\le 2.5$ (ensures the shape is circular/elliptical, not thin horizontal/vertical line reflections).
* **Fill Ratio:** $\ge 0.40$ (ensures the component is compact and solid rather than sparse background noise).
* **Core Peak Intensity:** Maximum pixel intensity inside the blob must be $\ge 240$.
* **Overall Brightness:** Average pixel intensity inside the blob must be $\ge 210$.

If any blob satisfies all of the above criteria, the blue light is classified as ON. This approach is completely location-invariant, working seamlessly if the camera moves, pans, or shifts.

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
