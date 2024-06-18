package e2e

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = describe("DNS working", func() {
	f := framework.NewDefaultFramework("dns")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelBaseline
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	f.It("Should run a pod on a new node to verify DNS daemonset [Zalando] [DNS]", f.WithSlow(), func(ctx context.Context) {
		ns := f.Namespace.Name
		nameprefix := "dns-test-"
		labels := map[string]string{
			"application": "dns-check",
		}

		By("Creating a pod which runs dns-checker on a new node")
		pod := createDNSCheckPod(nameprefix, ns, labels)
		_, err := cs.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Could not create POD %s", pod.Name)
		framework.ExpectNoError(waitForPodSuccessInNamespaceTimeout(ctx, f.ClientSet, pod.Name, pod.Namespace, 10*time.Minute))
	})
})

// waitForPodSuccessInNamespaceTimeout returns nil if the pod reached state success, or an error if it reached failure or ran too long.
// This is a copy of the upstream function but using `gomega.StopTrying` to fail early if the pod failed.
func waitForPodSuccessInNamespaceTimeout(ctx context.Context, c clientset.Interface, podName, namespace string, timeout time.Duration) error {
	return e2epod.WaitForPodCondition(ctx, c, namespace, podName, fmt.Sprintf("%s or %s", v1.PodSucceeded, v1.PodFailed), timeout, func(pod *v1.Pod) (bool, error) {
		if pod.DeletionTimestamp == nil && pod.Spec.RestartPolicy == v1.RestartPolicyAlways {
			return true, fmt.Errorf("pod %q will never terminate with a succeeded state since its restart policy is Always", podName)
		}

		switch pod.Status.Phase {
		case v1.PodSucceeded:
			return true, nil
		case v1.PodFailed:
			return true, gomega.StopTrying(fmt.Sprintf("pod %q failed with status: %+v", podName, pod.Status))
		default:
			return false, nil
		}
	})
}
