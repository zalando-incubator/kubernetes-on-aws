#!/bin/bash
set -euo pipefail

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

export CLUSTER_ALIAS="${CLUSTER_ALIAS:-"e2e-test"}"
# TODO: we need the date in LOCAL_ID because of CDP retriggering
export LOCAL_ID="${LOCAL_ID:-"e2e-$CDP_BUILD_VERSION-$(date +'%H%M%S')"}"
export API_SERVER_URL="https://${LOCAL_ID}.${HOSTED_ZONE}"
export INFRASTRUCTURE_ACCOUNT="aws:${AWS_ACCOUNT}"
export ETCD_ENDPOINTS="${ETCD_ENDPOINTS:-"etcd-server.etcd.${HOSTED_ZONE}:2379"}"
export CLUSTER_ID="${INFRASTRUCTURE_ACCOUNT}:${REGION}:${LOCAL_ID}"
export WORKER_SHARED_SECRET="${WORKER_SHARED_SECRET:-"$(pwgen 30 -n1)"}"

echo "Creating cluster ${CLUSTER_ID}: ${API_SERVER_URL}"

# TODO drop later
export MASTER_PROFILE="master"
export WORKER_PROFILE="worker"

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
export S3_AWS_IAM_BUCKET="zalando-e2e-test-${AWS_ACCOUNT}-${LOCAL_ID}"
export AWS_IAM_ROLE="${LOCAL_ID}-e2e-aws-iam-test"

# wait for resouces to be ready
# TODO: make a feature of CLM --wait-for-kube-system
"./wait-for-update.py" --timeout 1200

# Run e2e tests
# * conformance tests
# * statefulset tests
# * custom 'zalando' tests
#
# Disable DNS tests covering DNS names of format: <name>.<namespace>.svc which
# we don't support with the ndots:2 configuration:
#
# * "should resolve DNS of partial qualified names for the cluster [DNS] [Conformance]"
#   https://github.com/kubernetes/kubernetes/blob/66049e3b21efe110454d67df4fa62b08ea79a19b/test/e2e/network/dns.go#L71-L98
#
# * "should resolve DNS of partial qualified names for services"
#   https://github.com/kubernetes/kubernetes/blob/66049e3b21efe110454d67df4fa62b08ea79a19b/test/e2e/network/dns.go#L173-L220

# Disable Tests for setups which we don't support
#
# These are disabled because they assume nodePorts are reachable via the public
# IP of the node, we don't currently support that.
#
# * "[Fail] [sig-network] Services [It] should be able to change the type from ExternalName to NodePort [Conformance]"
#   https://github.com/kubernetes/kubernetes/blob/224be7bdce5a9dd0c2fd0d46b83865648e2fe0ba/test/e2e/network/service.go#L1037
# * "[Fail] [sig-network] Services [It] should be able to create a functioning NodePort service [Conformance]"
#   https://github.com/kubernetes/kubernetes/blob/224be7bdce5a9dd0c2fd0d46b83865648e2fe0ba/test/e2e/network/service.go#L551
ginkgo -nodes=25 -flakeAttempts=2 \
    -focus="(\[Conformance\]|\[StatefulSetBasic\]|\[Feature:StatefulSet\]\s\[Slow\].*mysql|\[Zalando\])" \
    -skip="(\[Serial\])" \
    -skip="(should.resolve.DNS.of.partial.qualified.names.for.the.cluster|should.provide.DNS.for.services|should.be.able.to.change.the.type.from.ExternalName.to.NodePort|should.be.able.to.create.a.functioning.NodePort.service|\[Serial\])" \
    "e2e.test" -- -delete-namespace-on-failure=false -allowed-not-ready-nodes=1
# TODO: drop the `-allowed-not-ready-nodes=1` flag in v1.17, fixed by https://github.com/kubernetes/kubernetes/commit/877218768019e104f64fbac13307b66ab8c0ba3e

# delete cluster
clm decommission \
    --remove-volumes \
    --token="${WORKER_SHARED_SECRET}" \
    --directory="$(pwd)/../.." \
    --assumed-role=cluster-lifecycle-manager-entrypoint \
    --debug \
    --registry=head_cluster.yaml
