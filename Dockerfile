ARG TARGETOS=freebsd

# Stage 1: Build the Go binary using Linux golang image
FROM golang:1.26-alpine AS builder
ARG TARGETOS
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=amd64 go build -o kitchen-camera .

# Stage 2a: Linux target image
FROM alpine:latest AS target-linux
RUN apk add --no-cache ffmpeg
COPY --from=builder /app/kitchen-camera /usr/local/bin/kitchen-camera
ENV RTSP_URI=""
ENV DETECTION_THRESHOLD="50"
ENTRYPOINT ["/usr/local/bin/kitchen-camera"]

# Stage 2b: FreeBSD target image
FROM freebsd:latest AS target-freebsd
RUN pkg update && pkg install -y ffmpeg
COPY --from=builder /app/kitchen-camera /usr/local/bin/kitchen-camera
ENV RTSP_URI=""
ENV DETECTION_THRESHOLD="50"
ENTRYPOINT ["/usr/local/bin/kitchen-camera"]

# Final selected target stage
FROM target-${TARGETOS}
