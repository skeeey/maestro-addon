SHELL :=/bin/bash

container_tool?=podman

IMAGE_REGISTRY?=quay.io/stolostron
IMAGE_TAG?=latest
IMAGE_NAME?=$(IMAGE_REGISTRY)/maestro-addon:$(IMAGE_TAG)
PERFORMANCE_TOOL_IMAGE_NAME?=$(IMAGE_REGISTRY)/maestro-per-tool:$(IMAGE_TAG)

GOLANGCI_LINT_VERSION=v1.54.1

verify:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	go vet ./...
	golangci-lint run --timeout=3m ./...
.PHONY: verify

build:
	go build -trimpath -ldflags="-s -w" -o maestroaddon cmd/manager/main.go
.PHONY: build

build-performance-tool:
	go build -trimpath -ldflags="-s -w" -tags=kafka -o maestroperf test/performance/cmd/main.go
.PHONY: build-performance-tool

image:
	$(container_tool) build -f Dockerfile -t $(IMAGE_NAME) .
.PHONY: image

performance-tool-image:
	$(container_tool) build -f Dockerfile.perf -t $(PERFORMANCE_TOOL_IMAGE_NAME) .
.PHONY: performance-tool-image

test:
	go test ./pkg/...
.PHONY: test

e2e-test:
	go test -c ./test/e2e -o _output/e2e.test
	./test/e2e/hack/e2e.sh
.PHONY: e2e-test

cleanup:
	rm -rf _output
.PHONY: cleanup
