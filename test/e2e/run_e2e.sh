#!/bin/bash
set -euo pipefail

create_cluster=false
e2e=false
stackset_e2e=false
decommission_cluster=false
COMMAND="${1:-"all"}" # all, create-cluster, e2e, stackset-e2e, decommission-cluster

case "$COMMAND" in
    all)
        create_cluster=true
        e2e=true
        stackset_e2e=true
        decommission_cluster=true
        ;;
    create-cluster)
        create_cluster=true
        ;;
    e2e)
        e2e=true
        ;;
    stackset-e2e)
        stackset_e2e=true
        ;;
    decommission-cluster)
        decommission_cluster=true
        ;;
    *)
        echo "Unknown command: $COMMAND"
        exit 1
esac

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
RESULT_BUCKET="${RESULT_BUCKET:-""}"

export CLUSTER_ALIAS="${CLUSTER_ALIAS:-"teapot-e2e"}"
export LOCAL_ID="${LOCAL_ID:-"e2e-${CDP_BUILD_VERSION}"}"
export API_SERVER_URL="https://${LOCAL_ID}.${HOSTED_ZONE}"
export INFRASTRUCTURE_ACCOUNT="aws:${AWS_ACCOUNT}"
export ETCD_ENDPOINTS="${ETCD_ENDPOINTS:-"http://etcd-server.etcd.${HOSTED_ZONE}:2379"}"
export CLUSTER_ID="${INFRASTRUCTURE_ACCOUNT}:${REGION}:${LOCAL_ID}"
export WORKER_SHARED_SECRET="${WORKER_SHARED_SECRET:-"$(pwgen 30 -n1)"}"

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

if [ "$create_cluster" = true ]; then
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
            --debug \
            --registry=base_cluster.yaml
    fi

    # generate updated clusters.yaml
    "./cluster_config.sh" "${CDP_HEAD_COMMIT_ID}" "ready" > head_cluster.yaml
    # Update cluster
    clm provision \
        --token="${WORKER_SHARED_SECRET}" \
        --directory="$(pwd)/../.." \
        --debug \
        --registry=head_cluster.yaml

    # wait for resouces to be ready
    # TODO: make a feature of CLM --wait-for-kube-system
    "./wait-for-update.py" --timeout 1200
fi

if [ "$e2e" = true ]; then
    echo "Running e2e against cluster ${CLUSTER_ID}: ${API_SERVER_URL}"
    # disable cluster downscaling before running e2e
    "./cluster_config.sh" "${CDP_HEAD_COMMIT_ID}" "ready" "false" > cluster.yaml
    clm provision \
        --token="${WORKER_SHARED_SECRET}" \
        --directory="$(pwd)/../.." \
        --debug \
        --registry=cluster.yaml

    export S3_AWS_IAM_BUCKET="zalando-e2e-test-${AWS_ACCOUNT}-${LOCAL_ID}"
    export AWS_IAM_ROLE="${LOCAL_ID}-e2e-aws-iam-test"

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
    # * "should resolve DNS of partial qualified names for services [LinuxOnly]"
    #   https://github.com/kubernetes/kubernetes/blob/06ad960bfd03b39c8310aaf92d1e7c12ce618213/test/e2e/network/dns.go#L181-L234

    # Disable Tests for setups which we don't support
    #
    # These are disabled because they assume nodePorts are reachable via the public
    # IP of the node, we don't currently support that.
    #
    # * "[Fail] [sig-network] Services [It] should be able to change the type from ExternalName to NodePort [Conformance]"
    #   https://github.com/kubernetes/kubernetes/blob/224be7bdce5a9dd0c2fd0d46b83865648e2fe0ba/test/e2e/network/service.go#L1037
    # * "[Fail] [sig-network] Services [It] should be able to create a functioning NodePort service [Conformance]"
    #   https://github.com/kubernetes/kubernetes/blob/224be7bdce5a9dd0c2fd0d46b83865648e2fe0ba/test/e2e/network/service.go#L551
    # * "[Fail] [sig-network] Services [It] should have session affinity work for NodePort service [LinuxOnly] [Conformance]"
    #   https://github.com/kubernetes/kubernetes/blob/v1.19.2/test/e2e/network/service.go#L1813
    # * "[Fail] [sig-network] Services [It] should have session affinity timeout work for NodePort service [LinuxOnly] [Conformance]"
    #   https://github.com/kubernetes/kubernetes/blob/v1.19.2/test/e2e/network/service.go#L2522
    # * "[Fail] [sig-network] Services [It] should be able to switch session affinity for NodePort service [LinuxOnly] [Conformance]"
    #   https://github.com/kubernetes/kubernetes/blob/v1.19.2/test/e2e/network/service.go#L2538
    set +e

    mkdir -p junit_reports
    ginkgo -nodes=25 -flakeAttempts=2 \
        -focus="(\[Conformance\]|\[StatefulSetBasic\]|\[Feature:StatefulSet\]\s\[Slow\].*mysql|\[Zalando\])" \
        -skip="(should.resolve.DNS.of.partial.qualified.names.for.the.cluster|should.resolve.DNS.of.partial.qualified.names.for.services|should.be.able.to.change.the.type.from.ExternalName.to.NodePort|should.be.able.to.create.a.functioning.NodePort.service|should.have.session.affinity.work.for.NodePort.service|should.have.session.affinity.timeout.work.for.NodePort.service|should.be.able.to.switch.session.affinity.for.NodePort.service|\[Serial\]|Should.create.gradual.traffic.routes|Should.create.blue-green.routes)" \
        "e2e.test" -- -delete-namespace-on-failure=false -non-blocking-taints=node.kubernetes.io/role,nvidia.com/gpu -report-dir=junit_reports
    TEST_RESULT="$?"

    set -e

    if [[ -n "$RESULT_BUCKET" ]]; then
        # Prepare metadata.json
        jq --arg targetBranch "$CDP_TARGET_BRANCH" \
           --arg head "$CDP_HEAD_COMMIT_ID" \
           --arg buildVersion "$CDP_BUILD_VERSION" \
           --argjson prNumber "$CDP_PULL_REQUEST_NUMBER" \
           --arg author "$CDP_PULL_REQUEST_AUTHOR" \
           --argjson exitStatus "$TEST_RESULT" \
           -n \
           '{timestamp: now | todate, success: ($exitStatus == 0), targetBranch: $targetBranch, author: $author, prNumber: $prNumber, head: $head, version: $buildVersion }' \
           > junit_reports/metadata.json

        TARGET_DIR="$(printf "junit-reports/%04d-%02d/%s" "$(date +%Y)" "$(date +%V)" "$LOCAL_ID")"
        echo "Uploading test results to S3 ($TARGET_DIR)"
        aws s3 cp \
          --acl bucket-owner-full-control \
          --recursive \
          --quiet \
          junit_reports/ "s3://$RESULT_BUCKET/$TARGET_DIR/"
    fi

    # enable cluster downscaling after running e2e
    "./cluster_config.sh" "${CDP_HEAD_COMMIT_ID}" "ready" "true" > cluster_downscaling_enabled.yaml
    clm provision \
        --token="${WORKER_SHARED_SECRET}" \
        --directory="$(pwd)/../.." \
        --debug \
        --registry=cluster_downscaling_enabled.yaml > clm.log
    clm_exit="$?"
    if [ "$clm_exit" -gt 0 ]; then
        cat clm.log
    fi

    exit "$TEST_RESULT"
fi

if [ "$stackset_e2e" = true ]; then
    namespace="stackset-e2e-$(date +'%H%M%S')"
    kubectl create namespace "$namespace"
    E2E_NAMESPACE="${namespace}" ./stackset-e2e -test.parallel 64
fi

if [ "$decommission_cluster" = true ]; then
    # generate updated clusters.yaml
    "./cluster_config.sh" "${CDP_HEAD_COMMIT_ID}" "ready" > head_cluster.yaml
    # delete cluster
    clm decommission \
        --remove-volumes \
        --token="${WORKER_SHARED_SECRET}" \
        --directory="$(pwd)/../.." \
        --assumed-role=cluster-lifecycle-manager-entrypoint \
        --debug \
        --registry=head_cluster.yaml
fi
