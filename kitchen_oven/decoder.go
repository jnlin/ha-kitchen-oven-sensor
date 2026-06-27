package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	"os/exec"
)

// FrameData holds the raw components of an H.264 video frame.
type FrameData struct {
	SPS   []byte
	PPS   []byte
	NALUs [][]byte
}

// DecodeH264Frame takes H.264 SPS, PPS, and NALUs, formats them into a standard Annex-B
// stream, pipes it to an FFmpeg subprocess, and decodes the resulting JPEG image.
func DecodeH264Frame(ctx context.Context, fd FrameData) (image.Image, error) {
	var buf bytes.Buffer

	// Prepend Annex-B start codes [0, 0, 0, 1] to SPS, PPS, and each NALU
	if len(fd.SPS) > 0 {
		buf.Write([]byte{0, 0, 0, 1})
		buf.Write(fd.SPS)
	}
	if len(fd.PPS) > 0 {
		buf.Write([]byte{0, 0, 0, 1})
		buf.Write(fd.PPS)
	}
	for _, nalu := range fd.NALUs {
		if len(nalu) > 0 {
			buf.Write([]byte{0, 0, 0, 1})
			buf.Write(nalu)
		}
	}

	// -f h264 specifies H.264 input format from pipe:0
	// -vframes 1 captures 1 frame
	// -f image2 -c:v mjpeg encodes the frame as JPEG output to pipe:1 (stdout)
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-f", "h264",
		"-i", "pipe:0",
		"-vframes", "1",
		"-f", "image2",
		"-c:v", "mjpeg",
		"-",
	)

	cmd.Stdin = &buf
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg decode error: %w, stderr: %s", err, stderrBuf.String())
	}

	img, _, err := image.Decode(&stdoutBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JPEG from FFmpeg output: %w", err)
	}

	return img, nil
}
