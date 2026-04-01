IMAGE  := ghcr.io/mlbright/thejoined
TAG    := $(shell git describe --tags --always --dirty)

.PHONY: build push publish

build:
	docker build -t $(IMAGE):$(TAG) -t $(IMAGE):latest .

push:
	docker push $(IMAGE):$(TAG)
	docker push $(IMAGE):latest

publish: build push
