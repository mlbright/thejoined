GHCR_OWNER      ?= cpacketnetworks
GHCR_IMAGE      := ghcr.io/$(GHCR_OWNER)/thejoined
TAG             := $(shell git describe --tags --always --dirty)

ARM64_BINARY := rna-linux-arm64
AMD64_BINARY := rna-linux-amd64
RELEASE_REPO ?= cPacketNetworks/thejoined

.PHONY: build build-arm64 build-amd64 push publish publish-ghcr release login-dockerhub update-actions check-actions

build:
	docker build \
		--build-arg SOURCE_REPO=$(GHCR_OWNER)/thejoined \
		-t $(GHCR_IMAGE):$(TAG) \
		-t $(GHCR_IMAGE):latest \
		.

build-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		go build -trimpath -ldflags="-s -w" -o $(ARM64_BINARY) .

build-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -trimpath -ldflags="-s -w" -o $(AMD64_BINARY) .

push:
	docker push $(GHCR_IMAGE):$(TAG)
	docker push $(GHCR_IMAGE):latest

publish: build push

# Multi-arch (amd64+arm64) GHCR-only publish; requires a buildx builder
# (docker buildx create --use). Build and push are a single step because
# multi-platform images cannot be loaded into the local Docker daemon.
publish-ghcr:
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg SOURCE_REPO=$(GHCR_OWNER)/thejoined \
		-t $(GHCR_IMAGE):$(TAG) \
		-t $(GHCR_IMAGE):latest \
		--push \
		.

# Attach the per-arch binaries to a GitHub Release for the current tag.
# HEAD must be clean and exactly on a tag already pushed to $(RELEASE_REPO):
# --verify-tag aborts on an unpushed tag, so the release can never silently
# anchor to the default-branch tip instead of the tagged commit.
release: build-amd64 build-arm64
	@test -z "$$(git status --porcelain)" || { echo "release: working tree is dirty" >&2; exit 1; }
	@git describe --tags --exact-match >/dev/null 2>&1 || { echo "release: HEAD is not exactly on a tag" >&2; exit 1; }
	sha256sum $(AMD64_BINARY) $(ARM64_BINARY) > SHA256SUMS
	gh release create $(TAG) --verify-tag -R $(RELEASE_REPO) --generate-notes \
		$(AMD64_BINARY) $(ARM64_BINARY) SHA256SUMS

login-dockerhub:
	docker login -u mbrightcpacket

# Pin every GitHub Action in .github/ to the commit of its latest stable
# release; the script header explains the SHA-pinning rationale.
# check-actions only reports, exiting non-zero when a pin is stale.
update-actions:
	scripts/update-actions.sh

check-actions:
	scripts/update-actions.sh --check
