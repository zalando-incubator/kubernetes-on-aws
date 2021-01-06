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
    audittrail_root_account_role: ${AUDITTRAIL_ROOT_ACCOUNT_ROLE}
    apiserver_etcd_prefix: /registry-${LOCAL_ID}
    apiserver_business_partner_ids: ${APISERVER_BUSINESS_PARTNER_IDS}
    etcd_s3_backup_bucket: zalando-kubernetes-etcd-${AWS_ACCOUNT}-${REGION}
    etcd_endpoints: "${ETCD_ENDPOINTS}"
    image_policy: e2e
    ca_key_decompressed: ${CA_KEY}
    ca_cert_decompressed: ${CA_CERT}
    apiserver_key_decompressed: ${APISERVER_KEY}
    apiserver_cert_decompressed: ${APISERVER_CERT}
    worker_key: ${WORKER_KEY}
    worker_cert: ${WORKER_CERT}
    proxy_client_key: ${PROXY_CLIENT_KEY}
    proxy_client_cert: ${PROXY_CLIENT_CERT}
    kubelet_client_key: ${KUBELET_CLIENT_KEY}
    kubelet_client_cert: ${KUBELET_CLIENT_CERT}
    admission_controller_cert: ${ADMISSION_CONTROLLER_CERT}
    admission_controller_key: ${ADMISSION_CONTROLLER_KEY}
    vpa_webhook_key: ${VPA_WEBHOOK_KEY}
    vpa_webhook_cert: ${VPA_WEBHOOK_CERT}
    service_account_private_key: ${SERVICE_ACCOUNT_PRIVATE_KEY}
    vpa_enabled: "true"
    worker_shared_secret: ${WORKER_SHARED_SECRET}
    lightstep_token: "${LIGHTSTEP_TOKEN}"
    zmon_agent_replicas: '0'
    zmon_aws_agent_replicas: '0'
    zmon_redis_replicas: '0'
    zmon_scheduler_replicas: '0'
    zmon_worker_replicas: '0'
    node_pool_feature_enabled: "true"
    enable_rbac: "true"
    dynamodb_service_link_enabled: "false"
    skipper_ingress_cpu: 100m
    efs_id: ${EFS_ID}
    webhook_id: ${INFRASTRUCTURE_ACCOUNT}:${REGION}:kube-aws-test
    kube_aws_ingress_controller_nlb_enabled: "true"
    vm_dirty_bytes: 134217728
    vm_dirty_background_bytes: 67108864
    prometheus_tsdb_retention_size: enabled
    coredns_max_upsteam_concurrency: 30
    ebs_root_volume_size: "550" # required by the limitRanger e2e tests (needs 500Gi ephemoral storage) https://github.com/kubernetes/kubernetes/blob/v1.18.3/test/e2e/scheduling/limit_range.go#L59
    routegroups_validation: "enabled"
    stackset_routegroup_support_enabled: "true"
    stackset_ingress_source_switch_ttl: "1m"
  criticality_level: 1
  environment: e2e
  id: ${CLUSTER_ID}
  infrastructure_account: ${INFRASTRUCTURE_ACCOUNT}
  lifecycle_status: ${2}
  local_id: ${LOCAL_ID}
  node_pools:
  - discount_strategy: none
    instance_types: ["m5a.large"]
    name: default-master
    profile: master-default
    min_size: 1
    max_size: 2
  - discount_strategy: spot
    instance_types: ["m4.large", "m5.large", "m5.xlarge", "m4.xlarge"]
    name: default-worker-splitaz
    profile: worker-splitaz
    min_size: 0
    max_size: 21
    config_items:
      cpu_manager_policy: static
  - discount_strategy: spot
    instance_types: ["m4.large", "m5.large", "m5.xlarge", "m4.xlarge"]
    name: default-worker
    profile: worker-default
    min_size: 0
    max_size: 21
  - discount_strategy: spot
    instance_types: ["m4.large", "m5.large", "m5.xlarge", "m4.xlarge"]
    config_items:
      availability_zones: "eu-central-1a"
      scaling_priority: "-100"
    name: worker-limit-az
    profile: worker-splitaz
    min_size: 0
    max_size: 21
  - discount_strategy: spot
    instance_types: ["m5d.large", "m5d.xlarge", "m5d.2xlarge"]
    name: worker-instance-storage
    profile: worker-default
    min_size: 0
    max_size: 21
  - discount_strategy: spot
    instance_types: ["p3.2xlarge", "g2.2xlarge", "g3s.xlarge", "g3.4xlarge"]
    name: worker-gpu
    profile: worker-default
    min_size: 0
    max_size: 3
    config_items:
      availability_zones: "eu-central-1a"
      labels: zalando.org/nvidia-gpu=tesla
      taints: nvidia.com/gpu=present:NoSchedule
      scaling_priority: "-100"
  provider: zalando-aws
  region: ${REGION}
  owner: '${OWNER}'
EOF
