ARG TARGETOS=freebsd

# Stage 1a: Builder for Linux (amd64/arm64)
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder-linux
ARG TARGETARCH
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags="-w -s" -o kitchen-camera .

# Stage 1b: Builder for FreeBSD (native compilation inside FreeBSD Podman)
FROM freebsd/freebsd-runtime:15.1 AS builder-freebsd
RUN env ASSUME_ALWAYS_YES=yes pkg bootstrap && \
    env ASSUME_ALWAYS_YES=yes pkg update && \
    env ASSUME_ALWAYS_YES=yes pkg install -y go
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -ldflags="-w -s" -o kitchen-camera .

# Stage 2a: Linux target image
FROM alpine:latest AS target-linux
RUN apk add --no-cache ffmpeg
COPY --from=builder-linux /app/kitchen-camera /usr/local/bin/kitchen-camera
ENV RTSP_URI=""
ENV DETECTION_THRESHOLD="50"
ENTRYPOINT ["/usr/local/bin/kitchen-camera"]

# Stage 2b: FreeBSD target image
FROM freebsd/freebsd-runtime:15.1 AS target-freebsd
RUN env ASSUME_ALWAYS_YES=yes pkg bootstrap && \
    env ASSUME_ALWAYS_YES=yes pkg update && \
    env ASSUME_ALWAYS_YES=yes pkg install -y ffmpeg
COPY --from=builder-freebsd /app/kitchen-camera /usr/local/bin/kitchen-camera
ENV RTSP_URI=""
ENV DETECTION_THRESHOLD="50"
ENTRYPOINT ["/usr/local/bin/kitchen-camera"]

# Final selected target stage
FROM target-${TARGETOS}
