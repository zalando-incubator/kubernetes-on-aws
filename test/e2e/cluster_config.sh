#!/bin/bash
set -euo pipefail
set -x

cat <<EOF
clusters:
- alias: ${CLUSTER_ALIAS}
  api_server_url: ${API_SERVER_URL}
  channel: ${1}
  config_items:
    zmon_root_account_role: ${ZMON_ROOT_ACCOUNT_ROLE}
    experimental_new_etcd_stack: "true"
    audittrail_root_account_role: ${AUDITTRAIL_ROOT_ACCOUNT_ROLE}
    session_manager_destination_arn: ${SESSION_MANAGER_DESTINATION_ARN}
    apiserver_etcd_prefix: /registry-${LOCAL_ID}
    apiserver_business_partner_ids: ${APISERVER_BUSINESS_PARTNER_IDS}
    etcd_s3_backup_bucket: zalando-kubernetes-etcd-${AWS_ACCOUNT}-${REGION}
    etcd_endpoints: "${ETCD_ENDPOINTS}"
    etcd_client_ca_cert: "${ETCD_CLIENT_CA_CERT}"
    etcd_client_ca_key: "${ETCD_CLIENT_CA_KEY}"
    etcd_scalyr_key: "${ETCD_SCALYR_KEY}"
    etcd_dns_record_prefixes: "etcd-server.etcd"
    docker_meta_url: https://docker-meta.stups-test.zalan.do
    vpa_enabled: "true"
    lightstep_token: "${LIGHTSTEP_TOKEN}"
    okta_auth_issuer_url: "${OKTA_AUTH_ISSUER_URL}"
    zmon_agent_replicas: '0'
    zmon_aws_agent_replicas: '0'
    zmon_redis_replicas: '0'
    zmon_scheduler_replicas: '0'
    zmon_worker_replicas: '0'
    node_pool_feature_enabled: "true"
    enable_rbac: "true"
    skipper_ingress_refuse_payload: "refused-pattern-1[cf724afc]refused-pattern-2"
    efs_id: ${EFS_ID}
    webhook_id: ${INFRASTRUCTURE_ACCOUNT}:${REGION}:kube-aws-test
    kube_aws_ingress_controller_nlb_enabled: "true"
    nlb_switch: "pre"
    vm_dirty_bytes: 134217728
    vm_dirty_background_bytes: 67108864
    coredns_max_upsteam_concurrency: 30
    routegroups_validation: "enabled"
    stackset_routegroup_support_enabled: "true"
    stackset_ingress_source_switch_ttl: "1m"
    teapot_admission_controller_daemonset_reserved_cpu: "518m"
    okta_auth_client_id: "kubernetes.cluster.teapot-e2e"
    teapot_admission_controller_validate_pod_images_soft_fail_namespaces: "^kube-system$"
    eks_okta_identity_provider: "false" # disabled to speed up EKS cluster creation for e2e.
    skipper_open_policy_agent_enabled: "${SKIPPER_OPA_ENABLED}"
    skipper_open_policy_agent_styra_token: "${STYRA_TOKEN}"
    skipper_open_policy_agent_bucket_arn: "${SKIPPER_OPA_BUCKET_ARN}"
    skipper_open_policy_agent_observability_url: "${SKIPPER_OPA_OBSERVABILITY_URL}"
    skipper_open_policy_agent_bundles_url: "${SKIPPER_OPA_BUNDLES_URL}"
    eks_ip_family: "ipv6"
  criticality_level: 1
  environment: e2e
  id: ${CLUSTER_ID}
  infrastructure_account: ${INFRASTRUCTURE_ACCOUNT}
  lifecycle_status: ${2}
  local_id: ${LOCAL_ID}
  node_pools:
  $(if [ "${CLUSTER_PROVIDER}" == "zalando-eks" ]; then
cat <<EOFF
- config_items:
      labels: dedicated=cluster-seed
      taints: dedicated=cluster-seed:NoSchedule
    discount_strategy: none
    instance_types:
    - "m6i.xlarge"
    max_size: 99
    min_size: 2
    name: seed-worker
    profile: worker-combined
EOFF
else
cat <<EOFF
- discount_strategy: none
    instance_types: ["m6g.large"]
    name: default-master
    profile: master-default
    min_size: 1
    max_size: 2
EOFF
  fi)
  - discount_strategy: spot
    instance_types:
    - "c6i.large"
    - "m6i.large"
    - "r6i.large"
    - "c7i.large"
    - "m7i.large"
    - "r7i.large"
    config_items:
      availability_zones: "eu-central-1a"
      labels: dedicated=worker-limit-az
      taints: dedicated=worker-limit-az:NoSchedule
    name: worker-limit-az
    profile: worker-splitaz
    min_size: 0
    max_size: 21
  - name: default-karpenter
    profile: worker-karpenter
    discount_strategy: none
    max_size: 0
    min_size: 0
    instance_types:
    - default-for-karpenter
    config_items:
      scaling_priority: "100"
      consolidate_after: "5m"
  - name: karpenter-arm
    profile: worker-karpenter
    discount_strategy: none
    max_size: 0
    min_size: 0
    instance_types:
    - not-specified
    config_items:
      requirements: "- key: kubernetes.io/arch\n  operator: In\n  values:\n  - arm64\n"
      consolidate_after: "5m"
  - discount_strategy: spot
    instance_types:
    - "c7g.large"
    - "c6i.large"
    - "c6a.large"
    - "m6i.large"
    - "m6a.large"
    - "m6g.large"
    - "m7g.large"
    min_size: 0
    max_size: 9
    profile: worker-splitaz
    name: skipper-ingress-node
    config_items:
      labels: dedicated=skipper-ingress
      taints: dedicated=skipper-ingress:NoSchedule
  - discount_strategy: spot
    instance_types:
    - "default-for-karpenter"
    min_size: 0
    max_size: 0
    profile: worker-karpenter
    name: worker-node-tests
    config_items:
      labels: dedicated=node-tests
      taints: dedicated=node-tests:NoSchedule
      consolidate_after: "5m"
  - discount_strategy: spot
    instance_types:
    - "g4dn.xlarge"
    - "g4dn.2xlarge"
    - "g4dn.4xlarge"
    - "g5.xlarge"
    - "g5.2xlarge"
    - "g5.4xlarge"
    - "g5.8xlarge"
    - "g5.16xlarge"
    - "g6.xlarge"
    - "g6.2xlarge"
    - "g6.4xlarge"
    name: karpenter-gpu
    profile: worker-karpenter
    min_size: 0
    max_size: 0
    config_items:
      labels: zalando.org/nvidia-gpu=tesla
      taints: nvidia.com/gpu=present:NoSchedule
      consolidate_after: "5m"
  - discount_strategy: none
    instance_types:
    - "default-for-karpenter"
    min_size: 0
    max_size: 0
    profile: worker-karpenter
    name: node-reboot-tests
    config_items:
      labels: dedicated=node-reboot-tests
      taints: dedicated=node-reboot-tests:NoSchedule
      consolidate_after: "5m"
  provider: ${CLUSTER_PROVIDER}
  region: ${REGION}
  owner: '${OWNER}'
EOF
