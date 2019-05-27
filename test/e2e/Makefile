.PHONY: clean build.docker build.push

BINARY       ?= kubernetes-on-aws-e2e
VERSION      ?= $(shell git describe --tags --always --dirty)
KUBE_VERSION ?= v1.14.3
IMAGE        ?= registry-write.opensource.zalan.do/teapot/$(BINARY)
TAG          ?= $(VERSION)
DOCKERFILE   ?= Dockerfile
MOD_DIR      ?= e2e_modules
MOD_PATH     ?= ../$(MOD_DIR)

default: build

# In case the $MOD_DIR doesn't exist, creat it at $MOD_PATH
$(MOD_PATH):
	mkdir -p $(MOD_PATH)

fix-go-modules:	$(MOD_PATH)
	MOD_PATH=$(MOD_PATH) ./setup_modules.sh

deps: fix-go-modules
	GO111MODULE=on go get github.com/onsi/ginkgo/ginkgo@v1.8.0

e2e.test: fix-go-modules
	GO111MODULE=on go test -v -c -o e2e.test

build: e2e.test

build.docker: build
	docker build --rm --build-arg KUBE_VERSION=$(KUBE_VERSION) -t "$(IMAGE):$(TAG)" -f $(DOCKERFILE) ../..

build.push: build.docker
	docker push "$(IMAGE):$(TAG)"

clean:
	rm -rf e2e.test
	rm -rf $(MOD_PATH)
