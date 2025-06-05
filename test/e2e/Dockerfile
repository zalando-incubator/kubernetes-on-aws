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
    printf "e1681d7addac6d1a192d17d8989764fb8f1143b1bc568de491f757313f6836c8c3180c0ad0d101679f9ac6a27c447865977d691df0db642390923082b5b4024d kubernetes-client-linux-amd64.tar.gz\n47692ea55565da56cddcb57820cd36d586d3a785dadc529d73ab659a904169aa15556d094ace5d2d40e47173c223af98203760abc53f361311bb1496734f6605 kubernetes-client-linux-arm64.tar.gz" | grep "${TARGETARCH}" | sha512sum -c - && \
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
