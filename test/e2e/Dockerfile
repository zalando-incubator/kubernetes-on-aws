# builder image
FROM golang:1.26 AS builder

RUN CGO_ENABLED=0 go install github.com/onsi/ginkgo/v2/ginkgo@v2.27.2

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
    printf "f68e81568b0356e81d27073876356f5516b84f3bc55ddfd7ebab199045b82a720e2adbac7124e800b8b705d1647721b1a529ba524b982d4ff433f62f26f5cdc9 kubernetes-client-linux-amd64.tar.gz\n81c427328605de156d45c0378ef6949c74255b34a22c370f11d16ded14202be463968e17be5334327df600fa40c1f42d6d27766968d8a57f85c0d25f240695b5 kubernetes-client-linux-arm64.tar.gz" | grep "${TARGETARCH}" | sha512sum -c - && \
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
