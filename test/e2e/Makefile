.PHONY: clean build.docker build.push

BINARY       ?= kubernetes-on-aws-e2e
VERSION      ?= $(shell git describe --tags --always --dirty)
KUBE_VERSION ?= v1.19.4
IMAGE        ?= registry-write.opensource.zalan.do/teapot/$(BINARY)
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
	GO111MODULE=on go get github.com/onsi/ginkgo/ginkgo@v1.8.0

e2e.test: generate-code
	GO111MODULE=on go test -v -c -o e2e.test

stackset-e2e:
	GO111MODULE=on CGO_ENABLED=0 go test -c -o stackset-e2e github.com/zalando-incubator/stackset-controller/cmd/e2e

build: e2e.test stackset-e2e

build.docker: build
	docker build --rm --build-arg KUBE_VERSION=$(KUBE_VERSION) -t "$(IMAGE):$(TAG)" -f $(DOCKERFILE) ../..

build.push: build.docker
	docker push "$(IMAGE):$(TAG)"

clean:
	rm -rf e2e.test
	rm -rf stackset-e2e
	rm -rf $(MOD_PATH)
	rm -rf build
