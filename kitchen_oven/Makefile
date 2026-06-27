GO ?= go
BINARY_NAME=kitchen-camera
IMAGE_NAME=kitchen-camera:latest

.PHONY: all build test docker-build clean

all: test build

build:
	$(GO) build -o $(BINARY_NAME) .

test:
	$(GO) test -v ./...

docker-build:
	@if [ "$$(uname)" = "FreeBSD" ] || command -v podman >/dev/null 2>&1; then \
		echo "Detected FreeBSD / Podman host. Building FreeBSD native image locally..."; \
		GOOS=freebsd GOARCH=amd64 $(GO) build -o kitchen-camera-bin .; \
		podman build -f Dockerfile.freebsd -t $(IMAGE_NAME) .; \
		rm -f kitchen-camera-bin; \
	elif command -v docker >/dev/null 2>&1; then \
		echo "Detected Docker host. Building Linux image in container..."; \
		docker build --build-arg TARGETOS=linux -t $(IMAGE_NAME) .; \
	else \
		echo "Error: Neither Docker nor Podman is installed." >&2; \
		exit 1; \
	fi

clean:
	rm -f $(BINARY_NAME) kitchen-camera-bin
