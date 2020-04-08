module github.com/zalando-incubator/kubernetes-on-aws/tests/e2e

require (
	github.com/NYTimes/gziphandler v1.0.1 // indirect
	github.com/cloudflare/cfssl v0.0.0-20190616170404-1bf3e59ec1cf // indirect
	github.com/containerd/console v0.0.0-20181022165439-0650fd9eeb50 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/evanphx/json-patch v4.2.0+incompatible
	github.com/gopherjs/gopherjs v0.0.0-20181103185306-d547d1d9531e // indirect
	github.com/gregjones/httpcache v0.0.0-20181110185634-c63ab54fda8f // indirect
	github.com/imdario/mergo v0.3.6 // indirect
	github.com/jteeuwen/go-bindata v0.0.0-20151023091102-a0ff2567cfb7
	github.com/karrick/godirwalk v1.8.0 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/onsi/ginkgo v1.10.1
	github.com/onsi/gomega v1.7.0
	github.com/opencontainers/runtime-spec v1.0.1 // indirect
	github.com/pquerna/ffjson v0.0.0-20181028064349-e517b90714f7 // indirect
	github.com/vishvananda/netns v0.0.0-20180720170159-13995c7128cc // indirect
	github.com/zalando-incubator/kube-aws-iam-controller v0.1.1
	gopkg.in/warnings.v0 v0.1.2 // indirect
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/apiserver v0.0.0
	k8s.io/client-go v10.0.0+incompatible
	k8s.io/kubernetes v1.16.8
)

replace k8s.io/kubernetes => ./e2e_modules/kubernetes

replace k8s.io/api => ./e2e_modules/kubernetes/staging/src/k8s.io/api

replace k8s.io/apiextensions-apiserver => ./e2e_modules/kubernetes/staging/src/k8s.io/apiextensions-apiserver

replace k8s.io/apimachinery => ./e2e_modules/kubernetes/staging/src/k8s.io/apimachinery

replace k8s.io/apiserver => ./e2e_modules/kubernetes/staging/src/k8s.io/apiserver

replace k8s.io/cli-runtime => ./e2e_modules/kubernetes/staging/src/k8s.io/cli-runtime

replace k8s.io/client-go => ./e2e_modules/kubernetes/staging/src/k8s.io/client-go

replace k8s.io/cloud-provider => ./e2e_modules/kubernetes/staging/src/k8s.io/cloud-provider

replace k8s.io/code-generator => ./e2e_modules/kubernetes/staging/src/k8s.io/code-generator

replace k8s.io/kube-aggregator => ./e2e_modules/kubernetes/staging/src/k8s.io/kube-aggregator

replace k8s.io/kube-controller-manager => ./e2e_modules/kubernetes/staging/src/k8s.io/kube-controller-manager

replace k8s.io/kube-proxy => ./e2e_modules/kubernetes/staging/src/k8s.io/kube-proxy

replace k8s.io/kube-scheduler => ./e2e_modules/kubernetes/staging/src/k8s.io/kube-scheduler

replace k8s.io/kubectl => ./e2e_modules/kubernetes/staging/src/k8s.io/kubectl

replace k8s.io/kubelet => ./e2e_modules/kubernetes/staging/src/k8s.io/kubelet

replace k8s.io/sample-apiserver => ./e2e_modules/kubernetes/staging/src/k8s.io/sample-apiserver

replace k8s.io/sample-cli-plugin => ./e2e_modules/kubernetes/staging/src/k8s.io/sample-cli-plugin

replace k8s.io/sample-controller => ./e2e_modules/kubernetes/staging/src/k8s.io/sample-controller

replace k8s.io/metrics => ./e2e_modules/kubernetes/staging/src/k8s.io/metrics

replace k8s.io/csi-translation-lib => ./e2e_modules/kubernetes/staging/src/k8s.io/csi-translation-lib

replace k8s.io/legacy-cloud-providers => ./e2e_modules/kubernetes/staging/src/k8s.io/legacy-cloud-providers

replace k8s.io/cluster-bootstrap => ./e2e_modules/kubernetes/staging/src/k8s.io/cluster-bootstrap

replace k8s.io/component-base => ./e2e_modules/kubernetes/staging/src/k8s.io/component-base

replace k8s.io/cri-api => ./e2e_modules/kubernetes/staging/src/k8s.io/cri-api

go 1.13
