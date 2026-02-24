# builder image
FROM golang:1.26 AS builder

RUN CGO_ENABLED=0 go install github.com/onsi/ginkgo/v2/ginkgo@v2.25.1

# final image
# TODO get rid of python dependencies
# * wait-for-update.py
FROM container-registry.zalando.net/library/python-3.14-slim:latest

ARG TARGETARCH

RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -qq bc curl git jq pwgen \
  && rm -rf /var/lib/apt/lists/* \
  && pip3 install awscli --no-cache-dir -q

ARG KUBE_VERSION
RUN curl -L -s --fail "https://dl.k8s.io/${KUBE_VERSION}/kubernetes-client-linux-${TARGETARCH}.tar.gz" -o "kubernetes-client-linux-${TARGETARCH}.tar.gz" && \
    printf "51a45687e5cc04620673d92cc31308ccfc60fa4fd4e0836369b32d98841d4617343dfb899e6c9dd4c665e85bf1fe55402ae234481a6d6bd9275fa1d89cfc676e kubernetes-client-linux-amd64.tar.gz\n5f29e3c44b5f6c98278664a3ee702fc17ad8f3347353030458aa065c5e19e4ce3ce5f826cdfc91b12dfa6b427877885a22516485dc135c880d7d42a781a8df7e kubernetes-client-linux-arm64.tar.gz" | grep "${TARGETARCH}" | sha512sum -c - && \
    tar xvf "kubernetes-client-linux-${TARGETARCH}.tar.gz" --strip-components 3 kubernetes/client/bin/ && \
    rm "kubernetes-client-linux-${TARGETARCH}.tar.gz" && \
    mv kubectl /usr/bin/kubectl


COPY --from=builder /go/bin/ginkgo /usr/local/bin/ginkgo

# copy CLM
COPY --from=container-registry.zalando.net/teapot/cluster-lifecycle-manager:latest /clm /usr/bin/clm
COPY --from=container-registry.zalando.net/teapot/aws-account-creator:latest /aws-account-creator /usr/bin/aws-account-creator

ADD test/e2e/build/linux/${TARGETARCH}/e2e.test /workdir/test/e2e/e2e.test
ADD test/e2e/build/linux/${TARGETARCH}/stackset-e2e /workdir/test/e2e/stackset-e2e
ADD test/e2e/build/linux/${TARGETARCH}/check-daemonset-updated /workdir/test/e2e/check-daemonset-updated
COPY test/e2e/*.sh /workdir/test/e2e/
COPY test/e2e/*.py /workdir/test/e2e/
ADD test/e2e/apply /workdir/test/e2e/apply
ADD test/e2e/loadtest /workdir/test/e2e/loadtest
ADD cluster /workdir/cluster

ENV KUBECTL_PATH=/usr/bin/kubectl
ENV KUBERNETES_PROVIDER=skeleton
ENV KUBERNETES_CONFORMANCE_TEST=Y

WORKDIR /workdir/test/e2e

ENTRYPOINT ["./run_e2e.sh"]
