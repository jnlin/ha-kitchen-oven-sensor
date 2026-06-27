.PHONY: all build test docker-build clean

BINARY_NAME=kitchen-camera
IMAGE_NAME=kitchen-camera:latest

all: test build

build:
	go build -o $(BINARY_NAME) .

test:
	go test -v ./...

docker-build:
	@if [ "$$(uname)" = "FreeBSD" ] || command -v podman >/dev/null 2>&1; then \
		echo "Detected FreeBSD / Podman host. Building FreeBSD native image..."; \
		podman build --build-arg BASE_IMAGE=freebsd:latest --build-arg TARGETOS=freebsd -t $(IMAGE_NAME) .; \
	elif command -v docker >/dev/null 2>&1; then \
		echo "Detected Docker host. Building Linux image..."; \
		docker build --build-arg BASE_IMAGE=alpine:latest --build-arg TARGETOS=linux -t $(IMAGE_NAME) .; \
	else \
		echo "Error: Neither Docker nor Podman is installed." >&2; \
		exit 1; \
	fi

clean:
	rm -f $(BINARY_NAME)
