package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	kubeapi "k8s.io/kubernetes/pkg/apis/core"
	admissionapi "k8s.io/pod-security-admission/api"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyconfigurationsautoscalingv1 "k8s.io/client-go/applyconfigurations/autoscaling/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/deployment"
)

var _ = describe("Infrastructure tests", func() {
	f := framework.NewDefaultFramework("zalando-kube-infra")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelBaseline
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("Mirror pods should be created for the main Kubernetes components [Zalando]", func() {
		for _, application := range []string{"kube-apiserver", "kube-controller-manager", "kube-scheduler"} {
			pods, err := podsForApplication(cs, application)
			framework.ExpectNoError(err)
			Expect(filterMirrorPods(pods)).NotTo(BeEmpty())
		}
	})

	It("All node pools should be able to run pods [Zalando]", func() {
		// When modifying this list, don't forget to modify cluster/manifests/e2e-resources/pool-reserve.yaml
		nodePools := []string{
			"default-worker-splitaz",
			"worker-combined",
			"worker-limit-az",
			"worker-instance-storage",
			"worker-node-tests",
			"worker-karpenter",
			"worker-arm64",
		}

		for _, pool := range nodePools {
			deploy, err := cs.AppsV1().Deployments("default").Get(context.Background(), fmt.Sprintf("pool-reserve-%s", pool), metav1.GetOptions{})
			framework.ExpectNoError(err)

			err = deployment.WaitForDeploymentComplete(cs, deploy)
			framework.ExpectNoError(err)

			// Scale out deployment to one more replica. In combination with Pod-Anti-Affinity, this should require one more node.
			_, err = cs.AppsV1().Deployments("default").ApplyScale(
				context.Background(),
				fmt.Sprintf("pool-reserve-%s", pool),
				applyconfigurationsautoscalingv1.Scale().WithSpec(applyconfigurationsautoscalingv1.ScaleSpec().WithReplicas(2)),
				metav1.ApplyOptions{FieldManager: "e2e.test", Force: true},
			)
			framework.ExpectNoError(err)
		}

		for _, pool := range nodePools {
			deploy, err := cs.AppsV1().Deployments("default").Get(context.Background(), fmt.Sprintf("pool-reserve-%s", pool), metav1.GetOptions{})
			framework.ExpectNoError(err)

			err = deployment.WaitForDeploymentComplete(cs, deploy)
			framework.ExpectNoError(err)
		}
	})

})

func podsForApplication(cs kubernetes.Interface, component string) ([]v1.Pod, error) {
	matchingPods, err := cs.CoreV1().Pods(kubeapi.NamespaceSystem).List(context.TODO(), metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{
			MatchLabels: map[string]string{
				"application": "kubernetes",
				"component":   component,
			},
		}),
	})
	if err != nil {
		return nil, err
	}
	return matchingPods.Items, nil
}

func filterMirrorPods(pods []v1.Pod) []v1.Pod {
	var result []v1.Pod
	for _, pod := range pods {
		if mirror := pod.Annotations["kubernetes.io/config.mirror"]; mirror != "" {
			result = append(result, pod)
		}
	}
	return result
}
