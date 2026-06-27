ARG TARGETOS=freebsd

# Stage 2a: Linux target image
FROM alpine:latest AS target-linux
RUN apk add --no-cache ffmpeg
COPY kitchen-camera-bin /usr/local/bin/kitchen-camera
ENV RTSP_URI=""
ENV DETECTION_THRESHOLD="50"
ENTRYPOINT ["/usr/local/bin/kitchen-camera"]

# Stage 2b: FreeBSD target image
FROM freebsd/freebsd-runtime:15.1 AS target-freebsd
RUN env ASSUME_ALWAYS_YES=yes pkg bootstrap && \
    env ASSUME_ALWAYS_YES=yes pkg update && \
    env ASSUME_ALWAYS_YES=yes pkg install -y ffmpeg
COPY kitchen-camera-bin /usr/local/bin/kitchen-camera
ENV RTSP_URI=""
ENV DETECTION_THRESHOLD="50"
ENTRYPOINT ["/usr/local/bin/kitchen-camera"]

# Final selected target stage
FROM target-${TARGETOS}
