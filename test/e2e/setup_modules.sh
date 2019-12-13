#!/bin/bash

KUBE_VERSION=${KUBE_VERSION:-""}
MOD_PATH="${MOD_PATH:-"./e2e_modules"}"

mkdir -p "$MOD_PATH"

if [[ -d "$MOD_PATH/kubernetes" ]]; then
    cd "$MOD_PATH/kubernetes"
    git fetch origin "refs/tags/${KUBE_VERSION}:refs/tags/${KUBE_VERSION}"
    git checkout -f "${KUBE_VERSION}"
else
    git clone --branch "${KUBE_VERSION}" --depth=1 https://github.com/kubernetes/kubernetes.git "$MOD_PATH/kubernetes"
fi
