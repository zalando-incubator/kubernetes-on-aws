#!/bin/bash
set -euo pipefail

create_cluster=false
e2e=false
loadtest_e2e=false
stackset_e2e=false
decommission_cluster=false
COMMAND="${1:-"all"}" # all, create-cluster, e2e, stackset-e2e, decommission-cluster

case "$COMMAND" in
    all)
        create_cluster=true
        e2e=true
        loadtest_e2e=true
        stackset_e2e=true
        decommission_cluster=true
        ;;
    create-cluster)
        create_cluster=true
        ;;
    e2e)
        e2e=true
        ;;
    loadtest-e2e)
        loadtest_e2e=true
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

E2E_SKIP_CLUSTER_UPDATE="${E2E_SKIP_CLUSTER_UPDATE:-"true"}"

# variables set for making it possible to run script locally
CDP_BUILD_VERSION="${CDP_BUILD_VERSION:-"local-1"}"
CDP_TARGET_REPOSITORY="${CDP_TARGET_REPOSITORY:-"github.com/zalando-incubator/kubernetes-on-aws"}"
CDP_TARGET_COMMIT_ID="${CDP_TARGET_COMMIT_ID:-"dev"}"
CDP_HEAD_COMMIT_ID="${CDP_HEAD_COMMIT_ID:-"$(git describe --tags --always)"}"
RESULT_BUCKET="${RESULT_BUCKET:-""}"

export CLUSTER_ALIAS="${CLUSTER_ALIAS:-"e2e-${CDP_BUILD_VERSION}"}"
export LOCAL_ID="${LOCAL_ID:-"e2e-${CDP_BUILD_VERSION}"}"
export API_SERVER_URL="https://${LOCAL_ID}.${HOSTED_ZONE}"
export INFRASTRUCTURE_ACCOUNT="aws:${AWS_ACCOUNT}"
export CLUSTER_ID="${INFRASTRUCTURE_ACCOUNT}:${REGION}:${LOCAL_ID}"
export CLUSTER_PROVIDER="${CLUSTER_PROVIDER:-"zalando-aws"}"

if [ "$CLUSTER_PROVIDER" == "zalando-aws" ]; then
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
    token: ${CLUSTER_ADMIN_TOKEN}
EOF

    KUBECONFIG="$(pwd)/kubeconfig"
    export KUBECONFIG="$KUBECONFIG"
fi

if [ "$create_cluster" = true ]; then
    echo "Creating cluster ${CLUSTER_ID}: ${API_SERVER_URL}"

    # TODO drop later
    # export MASTER_PROFILE="master"
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
            "./$BASE_CFG_PATH/test/e2e/cluster_config.sh" "${CDP_TARGET_COMMIT_ID}" "requested" > base_cluster.yaml
        else
            "./cluster_config.sh" "${CDP_TARGET_COMMIT_ID}" "requested" > base_cluster.yaml
        fi

        # generate the cluster certificates
        aws-account-creator refresh-certificates --registry-file base_cluster.yaml --create-ca

        # Create cluster
        clm provision \
            --token="${CLUSTER_ADMIN_TOKEN}" \
            --directory="$(pwd)/$BASE_CFG_PATH" \
            --debug \
            --registry=base_cluster.yaml \
            --manage-etcd-stack

        if [ "$CLUSTER_PROVIDER" == "zalando-eks" ]; then
            aws eks --region "${REGION}" update-kubeconfig --name "${LOCAL_ID}" --kubeconfig kubeconfig
            KUBECONFIG="$(pwd)/kubeconfig"
            export KUBECONFIG="$KUBECONFIG"
        fi

        # Wait for the resources to be ready
        ./wait-for-update.py --timeout 1200

        # provision and start load test
        echo "provision and start load test"
        ./start-load-test.sh --zone "$HOSTED_ZONE" --target "$(date +%s)" -v --timeout 900 --wait 30
    fi

    # generate updated clusters.yaml
    "./cluster_config.sh" "${CDP_HEAD_COMMIT_ID}" "ready" > head_cluster.yaml

    # either copy the certificates from the already created cluster or regenerate them from scratch
    if [ -f base_cluster.yaml ]; then
      ./copy-certificates.py base_cluster.yaml head_cluster.yaml
    else
      aws-account-creator refresh-certificates --registry-file head_cluster.yaml --create-ca --provider "${CLUSTER_PROVIDER}"
    fi

    # Update cluster
    echo "Updating cluster ${CLUSTER_ID}: ${API_SERVER_URL}"

    clm provision \
        --token="${CLUSTER_ADMIN_TOKEN}" \
        --directory="$(pwd)/../.." \
        --debug \
        --registry=head_cluster.yaml \
        --manage-etcd-stack

    aws eks --region "${REGION}" update-kubeconfig --name "${LOCAL_ID}" --kubeconfig kubeconfig
    KUBECONFIG="$(pwd)/kubeconfig"
    export KUBECONFIG="$KUBECONFIG"

    # rotate nodes with old daemonset pods and update strategy onDelete
    # This is important to ensure we e2e test against e.g. latest coredns daemonset
    ./check-daemonset-updated

    # Wait for the resources to be ready after the update
    # TODO: make a feature of CLM --wait-for-kube-system
    ./wait-for-update.py --timeout 1200

fi

if [ "$CLUSTER_PROVIDER" == "zalando-eks" ]; then
    aws eks --region "${REGION}" update-kubeconfig --name "${LOCAL_ID}" --kubeconfig kubeconfig
    KUBECONFIG="$(pwd)/kubeconfig"
    export KUBECONFIG="$KUBECONFIG"
fi

if [ "$e2e" = true ]; then
    echo "Running e2e against cluster ${CLUSTER_ID}: ${API_SERVER_URL}"
    # # disable cluster downscaling before running e2e
    # ./toggle-scaledown.py disable

    export S3_AWS_IAM_BUCKET="zalando-e2e-test-${AWS_ACCOUNT}-${LOCAL_ID}"
    export AWS_IAM_ROLE="${LOCAL_ID}-e2e-aws-iam-test"

    # Run e2e tests
    # * conformance tests
    # * statefulset tests
    # * custom 'zalando' tests
    #
    # Disable Tests for setups which we don't support
    #
    # These are disabled because hostPort is not supported in our
    # clusters yet. Currently there's no need to support it and
    # portMapping is not enabled in the Flannel CNI configmap.
    # * "[Fail] [sig-network] HostPort [It] validates that there is no conflict between pods with same hostPort but different hostIP and protocol [LinuxOnly] [Conformance]"
    #   https://github.com/kubernetes/kubernetes/blob/v1.31.0/test/e2e/network/hostport.go#L63
    set +e

    # TODO(linki): re-introduce the broken DNS record test after ExternalDNS handles it better
    #
    # This is still broken in external-dns:v0.14.2-master-40
    # InvalidChangeBatch: FATAL problem: DomainLabelEmpty (Domain label is empty) encountered with '_external-dns..teapot-e2e.zalan.do'
    #
    # introduce a broken DNS record to mess with ExternalDNS
    # kubectl apply -f broken-dns-record.yaml
    SKIPPED_TESTS=(
        "\[Serial\]"
        "validates.that.there.is.no.conflict.between.pods.with.same.hostPort.but.different.hostIP.and.protocol"
        "Should.create.gradual.traffic.routes"
    )

    if [ "$CLUSTER_PROVIDER" == "zalando-eks" ]; then
        # tests are skipped for eks because they test part of the control plane which is part of EKS
        SKIPPED_TESTS+=(
            "Mirror pods should be created for the main Kubernetes components \[Zalando\]"
            "Should audit API calls to create, update, patch, delete pods. \[Audit\] \[Zalando\]"
            "should validate permissions for \[Authorization\] \[RBAC\] \[Zalando\]" # TODO: temporary disabled because feature is missing
            #"Should NOT get AWS IAM credentials \[AWS-IAM\] \[Zalando\]" # TODO: check
            #"Should react to spot termination notices \[Zalando\] \[Spot\]" # TODO: check
            #"Should handle node restart \[Zalando\]" # TODO: check
            #"Should create DNS entry \[Zalando\]" # TODO: check
        )
    fi

    mkdir -p junit_reports
    ginkgo -procs=25 -flake-attempts=2 \
        -focus="(\[Conformance\]|\[StatefulSetBasic\]|\[Feature:StatefulSet\]\s\[Slow\].*mysql|\[Zalando\])" \
        -skip="($(IFS="|" ; echo "${SKIPPED_TESTS[*]}"))" \
        "e2e.test" -- \
        -delete-namespace-on-failure=false \
        -non-blocking-taints=node.kubernetes.io/role,nvidia.com/gpu,dedicated \
        -allowed-not-ready-nodes=-1 \
        -report-dir=junit_reports
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

        TARGET_DIR="$(printf "junit-reports/%04d-%02d/%s" "$(date +%Y)" "$(date +%-V)" "$LOCAL_ID")"
        echo "Uploading test results to S3 ($TARGET_DIR)"
        aws s3 cp \
          --acl bucket-owner-full-control \
          --recursive \
          --quiet \
          junit_reports/ "s3://$RESULT_BUCKET/$TARGET_DIR/"
    fi

    # enable cluster downscaling after running e2e
    ./toggle-scaledown.py enable

    exit "$TEST_RESULT"
fi

if [ "$stackset_e2e" = true ]; then
    namespace="stackset-e2e-$(date +'%H%M%S')"
    kubectl create namespace "$namespace"
    E2E_NAMESPACE="${namespace}" ./stackset-e2e -test.parallel 20
fi

if [ "$loadtest_e2e" = true ]; then
  >&2 echo "collect loadtest e2e data"
  prometheus=$(kubectl -n loadtest-e2e get ing prometheus -o json | jq -r '.spec.rules[0].host')

  >&2 echo "target prometheus: ${prometheus}"

  # get data for the last 30m
  curl --get -s -H"Accept: application/json" \
       --data-urlencode 'query=sum by(code) (rate(skipper_serve_host_count{application="e2e-vegeta"}[1m]))' \
       --data-urlencode "start=$(( $(date +%s) - (120*60) ))" \
       --data-urlencode "end=$(( $(date +%s) ))" \
       --data-urlencode "step=60" \
       "https://${prometheus}/api/v1/query_range" > /tmp/loadtest-e2e.json
  ls -l /tmp/loadtest-e2e.json
  cat /tmp/loadtest-e2e.json

  not_ok=$(jq -r '.data.result[] | select(.metric.code != "200") | .values[][1]' /tmp/loadtest-e2e.json \
    | awk 'BEGIN{cnt=0} {cnt=cnt+$1} END{print cnt}')
  ok=$(jq -r '.data.result[] | select(.metric.code == "200") | .values[][1]' /tmp/loadtest-e2e.json \
    | awk 'BEGIN{cnt=0} {cnt=cnt+$1} END{print cnt}')

  >&2 echo ""
  >&2 echo "DEBUG: e2e loadtest not OK: $not_ok"
  >&2 echo "DEBUG: e2e loadtest OK: $ok"

  if [ "${ok%.*}" -lt 1000 ]
  then
    >&2 echo "FAIL: e2e loadtest too few ok count $ok"
    exit 2
  elif [ "$( echo "scale=5; $not_ok / $ok > 0.000001" | bc )" -gt 0 ]; then
    >&2 echo "FAIL: e2e loadtest did not reach 99.999% OK rate"
    exit 2
  fi
fi

if [ "$decommission_cluster" = true ]; then
    describe_stack="$(aws --region "$REGION" cloudformation describe-stacks --stack-name "${LOCAL_ID}" --query "Stacks[0].Tags" 2>&1)"

    if [[ "$describe_stack" == *"${LOCAL_ID} does not exist"* ]]; then
      echo "Stack was already cleaned up"
      exit 0
    fi

    existing_tags="$(echo "$describe_stack" | jq --sort-keys -c '[.[] | {key: .Key, value: .Value}] | from_entries')"
    updated_tags="$(printf "%s" "$existing_tags" | jq --sort-keys -c '.["decommission-requested"] = "true"')"
    if [[ "$existing_tags" != "$updated_tags" ]]; then
        aws --region "$REGION" cloudformation update-stack --stack-name "${LOCAL_ID}" \
            --use-previous-template \
            --capabilities CAPABILITY_NAMED_IAM \
            --tags "$(printf "%s" "$updated_tags" | jq -c 'to_entries | [.[] | {Key: .key, Value: .value}]')"
    else
        echo "Stack already marked for decommissioning"
    fi
fi
