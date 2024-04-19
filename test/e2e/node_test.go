package e2e

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eevents "k8s.io/kubernetes/test/e2e/framework/events"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
)

var _ = describe("Node tests", func() {
	f := framework.NewDefaultFramework("node-tests")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	createTestPod := func(namespace string, nodePool string) *corev1.Pod {
		pausePod := nodeTestPod(namespace, nodePool, "pause")
		pausePod.Spec.Containers = []corev1.Container{pauseContainer()}
		pausePod.Spec.RestartPolicy = corev1.RestartPolicyNever

		By("Creating a test pod")
		pausePod, err := cs.CoreV1().Pods(namespace).Create(context.Background(), pausePod, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Could not create a test pod")
		framework.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(context.TODO(), f.ClientSet, pausePod.Name, pausePod.Namespace))

		pausePod, err = cs.CoreV1().Pods(namespace).Get(context.Background(), pausePod.Name, metav1.GetOptions{})
		framework.ExpectNoError(err, "Could not fetch the test pod")

		return pausePod
	}

	f.It("Should react to spot termination notices [Zalando] [Spot]", f.WithSlow(), func(ctx context.Context) {
		ns := f.Namespace.Name

		pausePod := createTestPod(ns, "node-tests")

		nodeName := pausePod.Spec.NodeName
		By("Ensuring that the node is schedulable initially")
		node, err := cs.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		framework.ExpectNoError(err, "Could not fetch the node")
		Expect(node.Spec.Unschedulable).To(BeFalse())

		By("Triggering the spot termination handler on the node")
		hostPathDirectory := corev1.HostPathDirectory
		terminationTriggerPodTemplate := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "termination-trigger-",
				Namespace:    ns,
			},
			Spec: corev1.PodSpec{
				Affinity:    nodeNameAffinity(nodeName),
				Tolerations: pausePod.Spec.Tolerations,
				Containers: []corev1.Container{
					{
						Name:  "mark-terminated",
						Image: awsCliImage,
						Command: []string{
							"/bin/sh",
							"-c",
							"echo test > /var/run/debug-spot-termination-notice",
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "var-run",
								ReadOnly:  false,
								MountPath: "/var/run",
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "var-run",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/var/run",
								Type: &hostPathDirectory,
							},
						},
					},
				},
				RestartPolicy: corev1.RestartPolicyNever,
			},
		}
		_, err = cs.CoreV1().Pods(ns).Create(ctx, terminationTriggerPodTemplate, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Could not create a termination trigger pod")

		By("Ensuring that pods are deleted from the node")
		framework.ExpectNoError(e2epod.WaitForPodNotFoundInNamespace(ctx, f.ClientSet, pausePod.Name, pausePod.Namespace, framework.PodDeleteTimeout))

		By("Ensuring that the ForceTerminatedSpot event is posted for affected pods")
		eventSelector := fmt.Sprintf("involvedObject.uid=%s,reason=ForceTerminatedSpot", pausePod.UID)
		framework.ExpectNoError(e2eevents.WaitTimeoutForEvent(ctx, f.ClientSet, pausePod.Namespace, eventSelector, "Deleted for spot termination", 30*time.Second))

		By("Ensuring that the node is unschedulable")
		node, err = cs.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		framework.ExpectNoError(err, "Could not fetch the node")
		Expect(node.Spec.Unschedulable).To(BeTrue())
	})

	f.It("Should handle kubelet restarts successfully [Zalando]", f.WithSlow(), func(ctx context.Context) {
		ns := f.Namespace.Name

		pausePod := createTestPod(ns, "node-tests")

		By(fmt.Sprintf("Restarting kubelet on node %s", pausePod.Spec.NodeName))
		boolTrue := true
		kubeletRestartPodTemplate := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "restart-kubelet-",
				Namespace:    ns,
			},
			Spec: corev1.PodSpec{
				Affinity:    nodeNameAffinity(pausePod.Spec.NodeName),
				Tolerations: pausePod.Spec.Tolerations,
				Containers: []corev1.Container{
					{
						Name:  "restart-kubelet",
						Image: framework.BusyBoxImage,
						Command: []string{
							"/bin/sh",
							"-c",
							`INITIAL="$(pgrep kubelet)" pkill -9 kubelet && sleep 60 && test "$INITIAL" != "$(pgrep kubelet)"`,
						},
						SecurityContext: &corev1.SecurityContext{
							Privileged: &boolTrue,
						},
					},
				},
				HostPID:       true,
				RestartPolicy: corev1.RestartPolicyNever,
			},
		}

		kubeletRestartPod, err := cs.CoreV1().Pods(ns).Create(ctx, kubeletRestartPodTemplate, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Could not create a kubelet restart pod")
		framework.ExpectNoError(e2epod.WaitForPodSuccessInNamespace(ctx, f.ClientSet, kubeletRestartPod.Name, kubeletRestartPod.Namespace))

		// Wait for a bit to give everything time to either fail completely or recover
		time.Sleep(1 * time.Minute)

		// Check that the node is still fine by running another pod on it
		testPodTemplate := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-pod-",
				Namespace:    ns,
			},
			Spec: corev1.PodSpec{
				Affinity:    nodeNameAffinity(pausePod.Spec.NodeName),
				Tolerations: pausePod.Spec.Tolerations,
				Containers: []corev1.Container{
					{
						Name:  "test",
						Image: framework.BusyBoxImage,
						Command: []string{
							"/bin/true",
						},
					},
				},
				RestartPolicy: corev1.RestartPolicyNever,
			},
		}
		testPod, err := cs.CoreV1().Pods(ns).Create(ctx, testPodTemplate, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Could not create a test pod")
		framework.ExpectNoError(e2epod.WaitForPodSuccessInNamespace(ctx, f.ClientSet, testPod.Name, testPod.Namespace))
	})
	f.It("Should handle node restart [Zalando]", f.WithSlow(), func(ctx context.Context) {
		ns := f.Namespace.Name

		pod := createTestPod(ns, "node-reboot-tests")
		nodeName := pod.Spec.NodeName
		gracefulSeconds := int64(0)

		boolTrue := true
		privilegedPodTemplate := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "privileged-pod-",
				Namespace:    ns,
			},
			Spec: corev1.PodSpec{
				Affinity:                      nodeNameAffinity(nodeName),
				Tolerations:                   pod.Spec.Tolerations,
				TerminationGracePeriodSeconds: &gracefulSeconds,
				Containers: []corev1.Container{
					{
						Name:    "privileged",
						Image:   framework.BusyBoxImage,
						Command: []string{"sh", "-c"},
						Args:    []string{"echo 1 > /proc/sys/kernel/sysrq; echo b > /proc/sysrq-trigger"},
						SecurityContext: &corev1.SecurityContext{
							Privileged: &boolTrue,
						},
					},
				},
				HostPID:       true,
				RestartPolicy: corev1.RestartPolicyNever,
			},
		}

		privilegedPod, err := cs.CoreV1().Pods(ns).Create(ctx, privilegedPodTemplate, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Could not create a test pod")

		By("Ensuring that node and its respective pods are terminated")
		framework.ExpectNoError(e2epod.WaitForPodNotFoundInNamespace(ctx, f.ClientSet, privilegedPod.Name, privilegedPod.Namespace, framework.PodDeleteTimeout))
		framework.ExpectNoError(e2epod.WaitForPodNotFoundInNamespace(ctx, f.ClientSet, pod.Name, pod.Namespace, framework.PodDeleteTimeout))

		_, err = cs.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		gomega.Expect(apierrors.IsNotFound(err)).To(gomega.BeTrue(), "node should not be found")
	})
})
