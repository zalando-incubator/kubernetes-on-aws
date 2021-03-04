module github.com/zalando-incubator/kubernetes-on-aws/tests/e2e

go 1.15

require (
	github.com/NYTimes/gziphandler v1.0.1 // indirect
	github.com/evanphx/json-patch v4.9.0+incompatible
	github.com/gopherjs/gopherjs v0.0.0-20181103185306-d547d1d9531e // indirect
	github.com/gregjones/httpcache v0.0.0-20181110185634-c63ab54fda8f // indirect
	github.com/jteeuwen/go-bindata v0.0.0-20151023091102-a0ff2567cfb7
	github.com/karrick/godirwalk v1.8.0 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/pkg/errors v0.9.1
	github.com/spf13/viper v1.4.0
	github.com/szuecs/routegroup-client v0.17.8-0.20200915193527-b33447c7d964
	github.com/zalando-incubator/kube-aws-iam-controller v0.1.2
	github.com/zalando-incubator/stackset-controller v1.3.24
	k8s.io/api v0.19.7
	k8s.io/apimachinery v0.19.7
	k8s.io/apiserver v0.0.0
	k8s.io/client-go v0.19.7
	k8s.io/kubernetes v0.0.0
)

replace (
	k8s.io/api => ./e2e_modules/kubernetes/staging/src/k8s.io/api
	k8s.io/apiextensions-apiserver => ./e2e_modules/kubernetes/staging/src/k8s.io/apiextensions-apiserver
	k8s.io/apimachinery => ./e2e_modules/kubernetes/staging/src/k8s.io/apimachinery
	k8s.io/apiserver => ./e2e_modules/kubernetes/staging/src/k8s.io/apiserver
	k8s.io/cli-runtime => ./e2e_modules/kubernetes/staging/src/k8s.io/cli-runtime
	k8s.io/client-go => ./e2e_modules/kubernetes/staging/src/k8s.io/client-go
	k8s.io/cloud-provider => ./e2e_modules/kubernetes/staging/src/k8s.io/cloud-provider
	k8s.io/cluster-bootstrap => ./e2e_modules/kubernetes/staging/src/k8s.io/cluster-bootstrap
	k8s.io/code-generator => ./e2e_modules/kubernetes/staging/src/k8s.io/code-generator
	k8s.io/component-base => ./e2e_modules/kubernetes/staging/src/k8s.io/component-base
	k8s.io/controller-manager => ./e2e_modules/kubernetes/staging/src/k8s.io/controller-manager
	k8s.io/cri-api => ./e2e_modules/kubernetes/staging/src/k8s.io/cri-api
	k8s.io/csi-translation-lib => ./e2e_modules/kubernetes/staging/src/k8s.io/csi-translation-lib
	k8s.io/kube-aggregator => ./e2e_modules/kubernetes/staging/src/k8s.io/kube-aggregator
	k8s.io/kube-controller-manager => ./e2e_modules/kubernetes/staging/src/k8s.io/kube-controller-manager
	k8s.io/kube-proxy => ./e2e_modules/kubernetes/staging/src/k8s.io/kube-proxy
	k8s.io/kube-scheduler => ./e2e_modules/kubernetes/staging/src/k8s.io/kube-scheduler
	k8s.io/kubectl => ./e2e_modules/kubernetes/staging/src/k8s.io/kubectl
	k8s.io/kubelet => ./e2e_modules/kubernetes/staging/src/k8s.io/kubelet
	k8s.io/kubernetes => ./e2e_modules/kubernetes
	k8s.io/legacy-cloud-providers => ./e2e_modules/kubernetes/staging/src/k8s.io/legacy-cloud-providers
	k8s.io/metrics => ./e2e_modules/kubernetes/staging/src/k8s.io/metrics
	k8s.io/sample-apiserver => ./e2e_modules/kubernetes/staging/src/k8s.io/sample-apiserver
	k8s.io/sample-cli-plugin => ./e2e_modules/kubernetes/staging/src/k8s.io/sample-cli-plugin
	k8s.io/sample-controller => ./e2e_modules/kubernetes/staging/src/k8s.io/sample-controller
)
