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
    printf "b09ee8f473b44ecd1689e80392dae203e4a0cce49fcd622e47e3b48c98afc78ce0600b6694d13903af4cffb059f90e0309772c4919de166bca30eacf40ee4273 kubernetes-client-linux-amd64.tar.gz\n6a224e4787d1877577a58cad1524b132229e1ccc9bd0eb588d0c1764ceba652280223fd1f693196a8d04f2792bbe50cb22b468b7b65b1bf97cf9a7d22263a3f4 kubernetes-client-linux-arm64.tar.gz" | grep "${TARGETARCH}" | sha512sum -c - && \
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
