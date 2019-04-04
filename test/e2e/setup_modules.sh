#!/bin/bash

MOD_PATH="${MOD_PATH:-"../e2e_modules"}"

mkdir -p "$MOD_PATH"

git clone --branch v1.12.7 --depth=1 \
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

# hack to force go modules to use the versions that compile
# tried to use `go get github.com/Azure/azure-sdk-for-go@v14.6.0` but go
# modules keep picking the latest release which doesn't work...
git clone --branch v19.0.0 --depth=1 \
    https://github.com/Azure/azure-sdk-for-go.git "$MOD_PATH/azure-sdk-for-go"
echo "module github.com/Azure/azure-sdk-for-go" > "$MOD_PATH/azure-sdk-for-go/go.mod"

git clone --branch v10.14.0 --depth=1 \
    https://github.com/Azure/go-autorest.git "$MOD_PATH/go-autorest"
echo "module github.com/Azure/go-autorest" > "$MOD_PATH/go-autorest/go.mod"

# generate bindata
go get github.com/jteeuwen/go-bindata/go-bindata@6025e8de665b31fa74ab1a66f2cddd8c0abf887e
"$MOD_PATH/kubernetes/hack/generate-bindata.sh"
