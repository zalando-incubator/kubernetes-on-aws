# builder image
FROM golang:1.21 as builder

RUN CGO_ENABLED=0 go install github.com/onsi/ginkgo/ginkgo@v1.16.5

# final image
# TODO get rid of python dependencies
# * wait-for-update.py
FROM container-registry.zalando.net/library/python-3.11-slim:latest

ARG TARGETARCH

RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y \
  bc \
  curl \
  git \
  jq \
  pwgen \
  && rm -rf /var/lib/apt/lists/* \
  && pip3 install awscli --no-cache-dir

ARG KUBE_VERSION
ADD https://storage.googleapis.com/kubernetes-release/release/$KUBE_VERSION/bin/linux/$TARGETARCH/kubectl /usr/bin/kubectl
RUN chmod +x /usr/bin/kubectl

COPY --from=builder /go/bin/ginkgo /usr/local/bin/ginkgo

# copy CLM
COPY --from=container-registry-test.zalando.net/teapot/cluster-lifecycle-manager:pr-744-5 /clm /usr/bin/clm
COPY --from=container-registry.zalando.net/teapot/aws-account-creator:latest /aws-account-creator /usr/bin/aws-account-creator

ADD . /workdir

ENV KUBECTL_PATH /usr/bin/kubectl
ENV KUBERNETES_PROVIDER skeleton
ENV KUBERNETES_CONFORMANCE_TEST Y

WORKDIR /workdir/test/e2e

ENTRYPOINT ["./run_e2e.sh"]
