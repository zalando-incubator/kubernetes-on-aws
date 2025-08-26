# builder image
FROM golang:1.24 AS builder

RUN CGO_ENABLED=0 go install github.com/onsi/ginkgo/v2/ginkgo@v2.21.0

# final image
# TODO get rid of python dependencies
# * wait-for-update.py
FROM container-registry.zalando.net/library/python-3.13-slim:latest

ARG TARGETARCH

RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -qq bc curl git jq pwgen \
  && rm -rf /var/lib/apt/lists/* \
  && pip3 install awscli --no-cache-dir -q

ARG KUBE_VERSION
RUN curl -L -s --fail "https://dl.k8s.io/${KUBE_VERSION}/kubernetes-client-linux-${TARGETARCH}.tar.gz" -o "kubernetes-client-linux-${TARGETARCH}.tar.gz" && \
    printf "e628239516ed6a3d07d47b451b7f42199fb5dcfb4d416f7b519235fd454e0fca3d0c273cc9c709f653a935a32c1f9fbd0a4be88f4c59d0ddcd674be2c289c8a5 kubernetes-client-linux-amd64.tar.gz\neb349a54d2013ae535fd60a0c32b0a932f176c9203541fba88e9eecbb794a2701479d09389e04950f5ed27b8a48383072b658cdfe7bddb3f0b60c2657a93d90f kubernetes-client-linux-arm64.tar.gz" | grep "${TARGETARCH}" | sha512sum -c - && \
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
