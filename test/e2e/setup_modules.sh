#!/bin/bash

KUBE_VERSION=${KUBE_VERSION:-""}
MOD_PATH="${MOD_PATH:-"./e2e_modules"}"

mkdir -p "$MOD_PATH"

git clone --branch "${KUBE_VERSION}" --depth=1 \
    https://github.com/kubernetes/kubernetes.git "$MOD_PATH/kubernetes"
