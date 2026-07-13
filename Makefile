GHCR_OWNER      ?= cpacketnetworks
GHCR_IMAGE      := ghcr.io/$(GHCR_OWNER)/thejoined
DOCKERHUB_IMAGE := cpacketnetworks/thejoined
TAG             := $(shell git describe --tags --always --dirty)

ARM64_BINARY := rna-linux-arm64

.PHONY: build build-arm64 push publish publish-ghcr login-dockerhub

build:
	docker build \
		-t $(GHCR_IMAGE):$(TAG) \
		-t $(GHCR_IMAGE):latest \
		-t $(DOCKERHUB_IMAGE):$(TAG) \
		-t $(DOCKERHUB_IMAGE):latest \
		.

build-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		go build -trimpath -ldflags="-s -w" -o $(ARM64_BINARY) .

push:
	docker push $(GHCR_IMAGE):$(TAG)
	docker push $(GHCR_IMAGE):latest
	docker push $(DOCKERHUB_IMAGE):$(TAG)
	docker push $(DOCKERHUB_IMAGE):latest

publish: build push

# Multi-arch (amd64+arm64) GHCR-only publish; requires a buildx builder
# (docker buildx create --use). Build and push are a single step because
# multi-platform images cannot be loaded into the local Docker daemon.
publish-ghcr:
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		-t $(GHCR_IMAGE):$(TAG) \
		-t $(GHCR_IMAGE):latest \
		--push \
		.

login-dockerhub:
	docker login -u mbrightcpacket
