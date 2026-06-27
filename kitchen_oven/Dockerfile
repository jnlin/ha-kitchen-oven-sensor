ARG TARGETOS=linux

# Stage 1: Build Go binaries statically for both platforms
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
ARG TARGETARCH
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags="-w -s" -o kitchen-camera-linux .
RUN CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -ldflags="-w -s" -o kitchen-camera-freebsd .

# Stage 2a: Linux target image
FROM alpine:latest AS target-linux
RUN apk add --no-cache ffmpeg
COPY --from=builder /app/kitchen-camera-linux /usr/local/bin/kitchen-camera
ENV RTSP_URI=""
ENV DETECTION_THRESHOLD="50"
ENTRYPOINT ["/usr/local/bin/kitchen-camera"]

# Stage 2b: FreeBSD target image
FROM freebsd/freebsd-runtime:15.1 AS target-freebsd
COPY --from=builder /app/kitchen-camera-freebsd /usr/local/bin/kitchen-camera
ENV RTSP_URI=""
ENV DETECTION_THRESHOLD="50"
COPY entrypoint-freebsd.sh /usr/local/bin/entrypoint-freebsd.sh
ENTRYPOINT ["/usr/local/bin/entrypoint-freebsd.sh"]

# Final selected target stage
FROM target-${TARGETOS}
