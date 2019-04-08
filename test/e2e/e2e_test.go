package e2e

import (
	"os"
	"testing"

	"k8s.io/kubernetes/test/e2e"
	"k8s.io/kubernetes/test/e2e/framework"

	// test sources
	_ "k8s.io/kubernetes/test/e2e/apimachinery"
	_ "k8s.io/kubernetes/test/e2e/apps"
	_ "k8s.io/kubernetes/test/e2e/auth"
	_ "k8s.io/kubernetes/test/e2e/autoscaling"
	_ "k8s.io/kubernetes/test/e2e/common"
	_ "k8s.io/kubernetes/test/e2e/instrumentation"
	_ "k8s.io/kubernetes/test/e2e/kubectl"
	_ "k8s.io/kubernetes/test/e2e/lifecycle"
	_ "k8s.io/kubernetes/test/e2e/lifecycle/bootstrap"
	_ "k8s.io/kubernetes/test/e2e/network"
	_ "k8s.io/kubernetes/test/e2e/node"
	_ "k8s.io/kubernetes/test/e2e/scalability"
	_ "k8s.io/kubernetes/test/e2e/scheduling"
	_ "k8s.io/kubernetes/test/e2e/servicecatalog"
	_ "k8s.io/kubernetes/test/e2e/storage"
	_ "k8s.io/kubernetes/test/e2e/ui"
)

func init() {
	framework.ViperizeFlags()
}

func TestE2E(t *testing.T) {
	e2e.RunE2ETests(t)
}

func e2eHostedZone() string {
	hostedZone := os.Getenv("HOSTED_ZONE")
	if hostedZone == "" {
		return "example.org"
	}
	return hostedZone
}
