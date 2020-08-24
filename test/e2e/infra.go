package e2e

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	kubeapi "k8s.io/kubernetes/pkg/apis/core"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
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

})

func podsForApplication(cs kubernetes.Interface, application string) ([]v1.Pod, error) {
	matchingPods, err := cs.CoreV1().Pods(kubeapi.NamespaceSystem).List(context.TODO(), metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{
			MatchLabels: map[string]string{"application": application},
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
