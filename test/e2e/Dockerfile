# builder image
FROM golang:1.25 AS builder

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
    printf "c0959e01d4d82b293848202b4392f8c784ca7e27abb68ebae36c303e34abb83378bcfa5679699c15681f1a3fc6eb801ea1204e086fc8e6512f11023fb1e178e7 kubernetes-client-linux-amd64.tar.gz\n1d49d8cb369d6f61a21d7616b01e64e1ae52ddc22f3c96b713a00603a0d28634fcd0d425e2d81ea0e5132a64c5843927941f84979ba5ea1a0a72eb198a1a9f8b kubernetes-client-linux-arm64.tar.gz" | grep "${TARGETARCH}" | sha512sum -c - && \
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
