#!/bin/bash

MOD_PATH="${MOD_PATH:-"../e2e_modules"}"

mkdir -p "$MOD_PATH"

git clone --branch v1.14.3 --depth=1 \
    https://github.com/kubernetes/kubernetes.git "$MOD_PATH/kubernetes"

# setup go.mod
echo "module k8s.io/kubernetes" > "$MOD_PATH/kubernetes/go.mod"

# initialize go.mod for modules to they can be used in the main go.mod replace
# directives
modules=(
api
apiextensions-apiserver
apimachinery
apiserver
cli-runtime
client-go
cloud-provider
code-generator
csi-api
kube-aggregator
kube-controller-manager
kube-proxy
kube-scheduler
kubelet
metrics
sample-apiserver
sample-cli-plugin
sample-controller
)

for m in "${modules[@]}"; do
    echo "module k8s.io/$m" > "$MOD_PATH/kubernetes/staging/src/k8s.io/$m/go.mod"
done

# generate bindata
go get github.com/jteeuwen/go-bindata/go-bindata@6025e8de665b31fa74ab1a66f2cddd8c0abf887e
"$MOD_PATH/kubernetes/hack/generate-bindata.sh"
