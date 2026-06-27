#!/bin/sh
if ! command -v ffmpeg >/dev/null 2>&1; then
    echo "ffmpeg not found, installing..."
    env ASSUME_ALWAYS_YES=yes pkg bootstrap
    env ASSUME_ALWAYS_YES=yes pkg update
    env ASSUME_ALWAYS_YES=yes pkg install -y ffmpeg
fi
exec /usr/local/bin/kitchen-camera "$@"
