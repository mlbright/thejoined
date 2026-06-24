GHCR_IMAGE      := ghcr.io/cpacketnetworks/thejoined
DOCKERHUB_IMAGE := cpacketnetworks/thejoined
TAG             := $(shell git describe --tags --always --dirty)

ARM64_BINARY := rna-linux-arm64

.PHONY: build build-arm64 push publish login-dockerhub

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

login-dockerhub:
	docker login -u mbrightcpacket
