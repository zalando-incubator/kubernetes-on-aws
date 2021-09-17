package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	kubeapi "k8s.io/kubernetes/pkg/apis/core"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/deployment"
)

var _ = framework.KubeDescribe("Infrastructure tests", func() {
	f := framework.NewDefaultFramework("zalando-kube-infra")
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("Mirror pods should be created for the main Kubernetes components [Zalando]", func() {
		for _, application := range []string{"kube-apiserver", "kube-controller-manager", "kube-scheduler"} {
			pods, err := podsForApplication(cs, application)
			Expect(err).NotTo(HaveOccurred())
			Expect(filterMirrorPods(pods)).NotTo(BeEmpty())
		}
	})

	It("All node pools should be able to run pods [Zalando]", func() {
		// When modifying this list, don't forget to modify cluster/manifests/e2e-resources/pool-reserve.yaml
		for _, pool := range []string{"default-worker-splitaz", "worker-combined", "worker-limit-az", "worker-instance-storage"} {
			deploy, err := cs.AppsV1().Deployments("default").Get(context.Background(), fmt.Sprintf("pool-reserve-%s", pool), metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			err = deployment.WaitForDeploymentComplete(cs, deploy)
			Expect(err).NotTo(HaveOccurred())
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
