.PHONY: clean build.docker build.push

BINARY       ?= kubernetes-on-aws-e2e
VERSION      ?= $(shell git describe --tags --always --dirty)
KUBE_VERSION ?= v1.37.2
IMAGE        ?= pierone.stups.zalan.do/teapot/$(BINARY)
SOURCES      = $(shell find . -name '*.go')
TAG          ?= $(VERSION)
DOCKERFILE   ?= Dockerfile

KUBE_MAJOR   = $(word 1,$(subst ., ,$(patsubst v%,%,$(KUBE_VERSION))))
KUBE_MINOR   = $(word 2,$(subst ., ,$(patsubst v%,%,$(KUBE_VERSION))))
VERSION_PKG  = k8s.io/component-base/version
E2E_LDFLAGS  = -X $(VERSION_PKG).gitMajor=$(KUBE_MAJOR) -X $(VERSION_PKG).gitMinor=$(KUBE_MINOR) -X $(VERSION_PKG).gitVersion=$(KUBE_VERSION)

default: build

deps:
	CGO_ENABLED=0 go install github.com/onsi/ginkgo/v2/ginkgo@v2.27.2

e2e.test: go.mod $(SOURCES)
	CGO_ENABLED=0 go test -v -c -ldflags '$(E2E_LDFLAGS)' -o e2e.test

stackset-e2e:
	CGO_ENABLED=0 go test -modfile stackset/go.mod -c -o stackset-e2e github.com/zalando-incubator/stackset-controller/cmd/e2e

check-daemonset-updated: go.mod daemonset-updated/main.go
	CGO_ENABLED=0 go build -trimpath -v -o $@ ./daemonset-updated

build: e2e.test stackset-e2e check-daemonset-updated

build/linux/amd64/e2e.test: go.mod $(SOURCES)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -v -c -ldflags '$(E2E_LDFLAGS)' -o $@

build/linux/amd64/stackset-e2e:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -modfile stackset/go.mod -c -o $@ github.com/zalando-incubator/stackset-controller/cmd/e2e

build/linux/amd64/check-daemonset-updated: go.mod daemonset-updated/main.go
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -v -o $@ ./daemonset-updated

build/linux/arm64/e2e.test: go.mod $(SOURCES)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go test -v -c -ldflags '$(E2E_LDFLAGS)' -o $@

build/linux/arm64/stackset-e2e:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go test -modfile stackset/go.mod -c -o $@ github.com/zalando-incubator/stackset-controller/cmd/e2e

build/linux/arm64/check-daemonset-updated: go.mod daemonset-updated/main.go
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -v -o $@ ./daemonset-updated

build.linux.amd64: build/linux/amd64/e2e.test build/linux/amd64/stackset-e2e build/linux/amd64/check-daemonset-updated
build.linux.arm64: build/linux/arm64/e2e.test build/linux/arm64/stackset-e2e build/linux/armd4/check-daemonset-updated

build.docker: build
	docker build --rm --build-arg KUBE_VERSION=$(KUBE_VERSION) -t "$(IMAGE):$(TAG)" -f $(DOCKERFILE) ../..

build.push: build.docker
	docker push "$(IMAGE):$(TAG)"

build.push.multiarch: build.linux.amd64 #build.linux.arm64
	docker buildx create --config /etc/cdp-buildkitd.toml --driver-opt network=host --bootstrap --use
	# docker buildx build --quiet --rm --build-arg KUBE_VERSION=$(KUBE_VERSION) -t "$(IMAGE):$(TAG)" --platform linux/amd64,linux/arm64 --push -f $(DOCKERFILE) ../..
	docker buildx build --quiet --rm --build-arg KUBE_VERSION=$(KUBE_VERSION) -t "$(IMAGE):$(TAG)" --platform linux/amd64 --push -f $(DOCKERFILE) ../..

clean:
	rm -rf e2e.test
	rm -rf stackset-e2e
	rm -rf check-daemonset-updated
	rm -rf build
