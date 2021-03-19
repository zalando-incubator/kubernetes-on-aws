package e2e

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eevents "k8s.io/kubernetes/test/e2e/framework/events"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = framework.KubeDescribe("Spot termination handler", func() {
	f := framework.NewDefaultFramework("spot-termination-handler")
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("Should react to spot termination notices [Slow] [Zalando] [Spot]", func() {
		ns := f.Namespace.Name
		poolName := "spot-termination-handler"

		tolerations := []corev1.Toleration{
			{
				Key:      "dedicated",
				Operator: corev1.TolerationOpEqual,
				Value:    poolName,
				Effect:   corev1.TaintEffectNoSchedule,
			},
		}

		pausePodTemplate := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "pause-",
				Namespace:    ns,
			},
			Spec: corev1.PodSpec{
				NodeSelector: map[string]string{
					"dedicated": poolName,
				},
				Tolerations:   tolerations,
				Containers:    []corev1.Container{pauseContainer()},
				RestartPolicy: corev1.RestartPolicyNever,
			},
		}
		By("Creating a test pod")
		pausePod, err := cs.CoreV1().Pods(ns).Create(context.Background(), pausePodTemplate, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Could not create a test pod")
		framework.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(f.ClientSet, pausePod.Name, pausePod.Namespace))

		pausePod, err = cs.CoreV1().Pods(ns).Get(context.Background(), pausePod.Name, metav1.GetOptions{})
		framework.ExpectNoError(err, "Could not fetch the test pod")

		nodeName := pausePod.Spec.NodeName
		By("Ensuring that the node is schedulable initially")
		node, err := cs.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
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
				Affinity: &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchFields: []corev1.NodeSelectorRequirement{
										{
											Key:      "metadata.name",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{nodeName},
										},
									},
								},
							},
						},
					},
				},
				Tolerations: tolerations,
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
		terminationTriggerPod, err := cs.CoreV1().Pods(ns).Create(context.Background(), terminationTriggerPodTemplate, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Could not create a termination trigger pod")
		framework.ExpectNoError(e2epod.WaitForPodSuccessInNamespace(f.ClientSet, terminationTriggerPod.Name, terminationTriggerPod.Namespace))

		By("Ensuring that pods are deleted from the node")
		framework.ExpectNoError(e2epod.WaitForPodToDisappear(f.ClientSet, pausePod.Namespace, pausePod.Name, labels.Everything(), framework.Poll, framework.PodDeleteTimeout))

		By("Ensuring that the ForceTerminatedSpot event is posted for affected pods")
		eventSelector := fmt.Sprintf("involvedObject.uid=%s,reason=ForceTerminatedSpot", pausePod.UID)
		framework.ExpectNoError(e2eevents.WaitTimeoutForEvent(f.ClientSet, pausePod.Namespace, eventSelector, "Deleted for spot termination", 30*time.Second))

		By("Ensuring that the node is unschedulable")
		node, err = cs.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
		framework.ExpectNoError(err, "Could not fetch the node")
		Expect(node.Spec.Unschedulable).To(BeTrue())
	})
})
