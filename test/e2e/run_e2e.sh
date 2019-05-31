#!/bin/bash
set -euo pipefail
set -x

E2E_SKIP_CLUSTER_UPDATE="${E2E_SKIP_CLUSTER_UPDATE:-"false"}"

# fetch internal configuration values
kubectl --namespace default get configmap teapot-kubernetes-e2e-config -o jsonpath='{.data.internal_config\.sh}' > internal_config.sh
# shellcheck disable=SC1091
source internal_config.sh

# variables set for making it possible to run script locally
CDP_BUILD_VERSION="${CDP_BUILD_VERSION:-"local-1"}"
CDP_TARGET_REPOSITORY="${CDP_TARGET_REPOSITORY:-"github.com/zalando-incubator/kubernetes-on-aws"}"
CDP_TARGET_COMMIT_ID="${CDP_TARGET_COMMIT_ID:-"dev"}"
CDP_HEAD_COMMIT_ID="${CDP_HEAD_COMMIT_ID:-"$(git describe --tags --always)"}"

# TODO: we need the date in LOCAL_ID because of CDP retriggering
LOCAL_ID="${LOCAL_ID:-"kube-e2e-$CDP_BUILD_VERSION-$(date +'%H%M%S')"}"
API_SERVER_URL="https://${LOCAL_ID}.${HOSTED_ZONE}"
INFRASTRUCTURE_ACCOUNT="aws:${AWS_ACCOUNT}"
ETCD_ENDPOINTS="${ETCD_ENDPOINTS:-"etcd-server.etcd.${HOSTED_ZONE}:2379"}"
CLUSTER_ID="${INFRASTRUCTURE_ACCOUNT}:${REGION}:${LOCAL_ID}"
WORKER_SHARED_SECRET="${WORKER_SHARED_SECRET:-"$(pwgen 30 -n1)"}"

export LOCAL_ID="$LOCAL_ID"
export API_SERVER_URL="$API_SERVER_URL"
export INFRASTRUCTURE_ACCOUNT="$INFRASTRUCTURE_ACCOUNT"
export ETCD_ENDPOINTS="$ETCD_ENDPOINTS"
export CLUSTER_ID="$CLUSTER_ID"
export WORKER_SHARED_SECRET="$WORKER_SHARED_SECRET"

# if E2E_SKIP_CLUSTER_UPDATE is true, don't create a cluster from base first
if [ "$E2E_SKIP_CLUSTER_UPDATE" != "true" ]; then
    BASE_CFG_PATH="base_config"

    # get head cluster config channel
    if [ -d "$BASE_CFG_PATH" ]; then
        rm -rf "$BASE_CFG_PATH"
    fi
    git clone "https://$CDP_TARGET_REPOSITORY" "$BASE_CFG_PATH"
    git -C "$BASE_CFG_PATH" reset --hard "${CDP_TARGET_COMMIT_ID}"

    # generate cluster.yaml
    # call the cluster_config.sh from base git checkout if possible
    if [ -f "$BASE_CFG_PATH/test/e2e/cluster_config.sh" ]; then
        "./$BASE_CFG_PATH/test/e2e/cluster_config.sh" \
        "${CDP_TARGET_COMMIT_ID}" "requested" > base_cluster.yaml
    else
        "./cluster_config.sh" "${CDP_TARGET_COMMIT_ID}" \
        "requested" > base_cluster.yaml
    fi

    # Create cluster
    clm provision \
        --token="${WORKER_SHARED_SECRET}" \
        --directory="$(pwd)/$BASE_CFG_PATH" \
        --assumed-role=cluster-lifecycle-manager-entrypoint \
        --debug \
        --registry=base_cluster.yaml
fi

# generate updated clusters.yaml
"./cluster_config.sh" "${CDP_HEAD_COMMIT_ID}" "ready" > head_cluster.yaml
# Update cluster
clm provision \
    --token="${WORKER_SHARED_SECRET}" \
    --directory="$(pwd)/../.." \
    --assumed-role=cluster-lifecycle-manager-entrypoint \
    --debug \
    --registry=head_cluster.yaml

# create kubeconfig
cat >kubeconfig <<EOF
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: ${API_SERVER_URL}
  name: e2e-cluster
contexts:
- context:
    cluster: e2e-cluster
    namespace: default
    user: e2e-bot
  name: e2e-cluster
current-context: e2e-cluster
preferences: {}
users:
- name: e2e-bot
  user:
    token: ${WORKER_SHARED_SECRET}
EOF

KUBECONFIG="$(pwd)/kubeconfig"
export KUBECONFIG="$KUBECONFIG"

# wait for resouces to be ready
# TODO: make a feature of CLM --wait-for-kube-system
"./wait-for-update.py" --timeout 1200

# Run e2e tests
# * conformance tests
# * statefulset tests
# * custom 'zalando' tests
#
# Broken e2e tests are disabled
#
# * "should provide DNS for the cluster [DNS] [Conformance]"
#   https://github.com/kubernetes/kubernetes/blob/release-1.13/test/e2e/network/dns.go#L48-L49
#   Fixed in v1.14.0
#
# * "should provide DNS for services [DNS] [Conformance]"
#   https://github.com/kubernetes/kubernetes/blob/release-1.13/test/e2e/network/dns.go#L105-L109
#   Fixed in v1.14.0
#
# * "should support remote command execution over websockets [NodeConformance] [Conformance]"
#   https://github.com/kubernetes/kubernetes/pull/73046
#   Fixed in v1.14.0
#
# * "should support retrieving logs from the container over websockets [NodeConformance] [Conformance]"
#   https://github.com/kubernetes/kubernetes/pull/73046
#   Fixed in v1.14.0
ginkgo -nodes=25 -flakeAttempts=2 \
    -focus="(\[Conformance\]|\[StatefulSetBasic\]|\[Feature:StatefulSet\]\s\[Slow\].*mysql|\[Zalando\])" \
    -skip="(\[Serial\])" \
    -skip="(should.provide.DNS.for.the.cluster|should.provide.DNS.for.services|should.support.retrieving.logs.from.the.container.over.websockets|should.support.remote.command.execution.over.websockets|\[Serial\])" \
    "e2e.test" -- -delete-namespace-on-failure=false

# delete cluster
clm decommission \
    --remove-volumes \
    --token="${WORKER_SHARED_SECRET}" \
    --directory="$(pwd)/../.." \
    --assumed-role=cluster-lifecycle-manager-entrypoint \
    --debug \
    --registry=head_cluster.yaml
