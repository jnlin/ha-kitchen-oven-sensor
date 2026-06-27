# Default parameters targeting FreeBSD
ARG TARGETOS=freebsd
ARG BASE_IMAGE=freebsd:latest

# Stage 1: Build the Go binary using Linux golang image
FROM golang:1.26-alpine AS builder
ARG TARGETOS
WORKDIR /app
COPY . .
# Disable CGO for a statically linked cross-compiled binary
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=amd64 go build -o kitchen-camera .

# Stage 2: Final container base
FROM ${BASE_IMAGE}
ARG TARGETOS

# Install ffmpeg using correct package manager depending on OS
RUN if [ "${TARGETOS}" = "freebsd" ]; then \
        pkg update && pkg install -y ffmpeg; \
    else \
        apk add --no-cache ffmpeg; \
    fi

# Copy the statically compiled binary to standard location
COPY --from=builder /app/kitchen-camera /usr/local/bin/kitchen-camera

# Environment defaults
ENV RTSP_URI=""
ENV DETECTION_THRESHOLD="50"

# Entrypoint setup
ENTRYPOINT ["/usr/local/bin/kitchen-camera"]
