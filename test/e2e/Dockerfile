# builder image
FROM golang:1.23 AS builder

RUN CGO_ENABLED=0 go install github.com/onsi/ginkgo/v2/ginkgo@v2.19.0

# final image
# TODO get rid of python dependencies
# * wait-for-update.py
FROM container-registry.zalando.net/library/python-3.12-slim:latest

ARG TARGETARCH

RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -qq bc curl git jq pwgen \
  && rm -rf /var/lib/apt/lists/* \
  && pip3 install awscli --no-cache-dir -q

ARG KUBE_VERSION
RUN curl -L -s --fail "https://dl.k8s.io/${KUBE_VERSION}/kubernetes-client-linux-${TARGETARCH}.tar.gz" -o "kubernetes-client-linux-${TARGETARCH}.tar.gz" && \
    printf "1db18b863f11d507a11c394ec7705495f7baf476db31a0da8392d28d41a1ca88dc0105d3064c35178d9cc3e047354a418662c077f123ae19e6042be19c65fee9 kubernetes-client-linux-amd64.tar.gz\ne974c1db451f095ea74a459dfd03585bd6cac95d038617f7ea3384925e90a194dfaa46fe2b8ffbade32a519ad349282194fa6764bede48a7ffca38753a10b6f9 kubernetes-client-linux-arm64.tar.gz" | grep "${TARGETARCH}" | sha512sum -c - && \
    tar xvf "kubernetes-client-linux-${TARGETARCH}.tar.gz" --strip-components 3 kubernetes/client/bin/ && \
    rm "kubernetes-client-linux-${TARGETARCH}.tar.gz" && \
    mv kubectl /usr/bin/kubectl


COPY --from=builder /go/bin/ginkgo /usr/local/bin/ginkgo

# copy CLM
COPY --from=container-registry.zalando.net/teapot/cluster-lifecycle-manager:latest /clm /usr/bin/clm
COPY --from=container-registry.zalando.net/teapot/aws-account-creator:latest /aws-account-creator /usr/bin/aws-account-creator

ADD . /workdir

ENV KUBECTL_PATH=/usr/bin/kubectl
ENV KUBERNETES_PROVIDER=skeleton
ENV KUBERNETES_CONFORMANCE_TEST=Y

WORKDIR /workdir/test/e2e

ENTRYPOINT ["./run_e2e.sh"]
