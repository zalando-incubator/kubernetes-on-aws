#!/bin/bash
az=$1
ver=$2

if [ -z "$az" ]; then
    echo 'Usage: ./create-stack.sh <AZ> <VERSION>'
    exit 1
fi

cluster=kube-aws-test
subnets_in_az=$(aws ec2 describe-subnets --filters "Name=availability-zone,Values=*$az" | jq '.Subnets|length')
if [ "$subnets_in_az" -eq 1 ]; then
    # we only have one subnet in this AZ (probably default VPC setup)
    subnet=$(aws ec2 describe-subnets --filters "Name=availability-zone,Values=*$az" | jq -r .Subnets[0].SubnetId)
else
    # choose the "public" subnet by name ("DMZ" subnet like http://docs.stups.io/en/latest/installation/aws-account-setup.html)
    subnet=$(aws ec2 describe-subnets --filters "Name=tag:Name,Values=dmz-*$az" | jq -r .Subnets[0].SubnetId)
fi
hosted_zone=$(aws route53 list-hosted-zones | jq -r .HostedZones[0].Name | sed s/\.$//)
etcd_discovery_domain=etcd.${hosted_zone}
api_server=https://kube-aws-test-${ver}.${hosted_zone}

key_arn=$(aws kms create-key --description "Kubernetes cluster $cluster $ver" | jq -r .KeyMetadata.Arn)
aws kms create-alias --alias-name "alias/${cluster}-${ver}" --target-key-id "$key_arn"
token=$(cat /dev/urandom | tr -dc _A-Z-a-z-0-9- | head -c 64)
# TODO: encrypt fixed token with KMS
userdata_master=$(cat userdata-master.yaml|sed -e s/STACK_VERSION/$ver/ -e s/ETCD_DISCOVERY_DOMAIN/$etcd_discovery_domain/ -e s,API_SERVER,$api_server, -e s/WORKER_SHARED_SECRET/$token/|gzip|base64)
userdata_worker=$(cat userdata-worker.yaml|sed -e s/STACK_VERSION/$ver/ -e s/ETCD_DISCOVERY_DOMAIN/$etcd_discovery_domain/ -e s,API_SERVER,$api_server, -e s/WORKER_SHARED_SECRET/$token/|gzip|base64)
senza create senza-definition.yaml $ver Subnet=$subnet UserDataMaster="$userdata_master" UserDataWorker="$userdata_worker" KmsKey="$key_arn"
