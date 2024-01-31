.PHONY: clean build.docker build.push

BINARY       ?= kubernetes-on-aws-e2e
VERSION      ?= $(shell git describe --tags --always --dirty)
KUBE_VERSION ?= v1.25.15
IMAGE        ?= pierone.stups.zalan.do/teapot/$(BINARY)
TAG          ?= $(VERSION)
DOCKERFILE   ?= Dockerfile
MOD_DIR      ?= e2e_modules
MOD_PATH     ?= ./$(MOD_DIR)
GO_BINDATA   = ./build/go-bindata

default: build

# In case the $MOD_DIR doesn't exist, creat it at $MOD_PATH
$(MOD_PATH):
	mkdir -p $(MOD_PATH)

$(GO_BINDATA):
	mkdir -p build
	GOBIN=$(shell pwd)/build go install github.com/jteeuwen/go-bindata/go-bindata

fix-go-modules:	$(MOD_PATH)
	MOD_PATH=$(MOD_PATH) KUBE_VERSION=$(KUBE_VERSION) ./setup_modules.sh

generate-code: fix-go-modules $(GO_BINDATA)
	make generated_files -C "$(MOD_PATH)/kubernetes"

deps: generate-code
	go install github.com/onsi/ginkgo/v2/ginkgo@v2.1.6

e2e.test: generate-code
	go test -v -c -o e2e.test

stackset-e2e:
	CGO_ENABLED=0 go test -modfile stackset/go.mod -c -o stackset-e2e github.com/zalando-incubator/stackset-controller/cmd/e2e

build: e2e.test stackset-e2e

build/linux/amd64/e2e.test: generate-code
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -v -c -o $@

build/linux/amd64/stackset-e2e:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -modfile stackset/go.mod -c -o $@ github.com/zalando-incubator/stackset-controller/cmd/e2e

build/linux/arm64/e2e.test: generate-code
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go test -v -c -o $@

build/linux/arm64/stackset-e2e:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go test -modfile stackset/go.mod -c -o $@ github.com/zalando-incubator/stackset-controller/cmd/e2e

build.linux.amd64: build/linux/amd64/e2e.test build/linux/amd64/stackset-e2e
build.linux.arm64: build/linux/arm64/e2e.test build/linux/arm64/stackset-e2e

build.docker: build
	docker build --rm --build-arg KUBE_VERSION=$(KUBE_VERSION) -t "$(IMAGE):$(TAG)" -f $(DOCKERFILE) ../..

build.push: build.docker
	docker push "$(IMAGE):$(TAG)"

build.push.multiarch: build.linux.amd64 #build.linux.arm64
	docker buildx create --config /etc/cdp-buildkitd.toml --driver-opt network=host --bootstrap --use
	# docker buildx build --rm --build-arg KUBE_VERSION=$(KUBE_VERSION) -t "$(IMAGE):$(TAG)" --platform linux/amd64,linux/arm64 --push -f $(DOCKERFILE) ../..
	docker buildx build --rm --build-arg KUBE_VERSION=$(KUBE_VERSION) -t "$(IMAGE):$(TAG)" --platform linux/amd64 --push -f $(DOCKERFILE) ../..

clean:
	rm -rf e2e.test
	rm -rf stackset-e2e
	rm -rf $(MOD_PATH)
	rm -rf build
