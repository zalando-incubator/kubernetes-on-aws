#!/bin/bash
ver=$1

if [ -z "$ver" ]; then
    echo 'Usage: ./create-stack.sh <VERSION>'
    exit 1
fi

cluster=kube-aws-test
hosted_zone=$(aws route53 list-hosted-zones | jq -r .HostedZones[0].Name | sed s/\.$//)
etcd_discovery_domain=etcd.${hosted_zone}
api_server=https://kube-aws-test-${ver}.${hosted_zone}

key_arn=$(aws kms create-key --description "Kubernetes cluster $cluster $ver" | jq -r .KeyMetadata.Arn)
aws kms create-alias --alias-name "alias/${cluster}-${ver}" --target-key-id "$key_arn"
token=$(cat /dev/urandom | LC_ALL=C tr -dc _A-Z-a-z-0-9- | head -c 64)
# TODO: encrypt fixed token with KMS
userdata_master=$(cat userdata-master.yaml|sed -e s/STACK_VERSION/$ver/ -e s/ETCD_DISCOVERY_DOMAIN/$etcd_discovery_domain/ -e s,API_SERVER,$api_server, -e s/WORKER_SHARED_SECRET/$token/ -e s/HOSTED_ZONE/$hosted_zone/ |gzip|base64 )
userdata_worker=$(cat userdata-worker.yaml|sed -e s/STACK_VERSION/$ver/ -e s/ETCD_DISCOVERY_DOMAIN/$etcd_discovery_domain/ -e s,API_SERVER,$api_server, -e s/WORKER_SHARED_SECRET/$token/|gzip|base64 )
senza create senza-definition.yaml $ver UserDataMaster="$userdata_master" UserDataWorker="$userdata_worker" KmsKey="$key_arn"
