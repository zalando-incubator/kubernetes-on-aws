package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = describe("Instance storage", func() {
	f := framework.NewDefaultFramework("instance-storage")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelBaseline
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("Should schedule a pod with ephemeral storage [Zalando]", func(ctx context.Context) {
		ns := f.Namespace.Name

		By("Creating a pod requesting 500Gi ephemeral storage")
		pod := createInstanceStorageTestPod("instance-storage-", ns)
		_, err := cs.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Could not create pod %s", pod.Name)

		framework.ExpectNoError(e2epod.WaitForPodSuccessInNamespace(ctx, cs, pod.Name, ns))
	})
})

func createInstanceStorageTestPod(nameprefix, namespace string) *v1.Pod {
	return &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameprefix + string(uuid.NewUUID()),
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{
				{
					Name:  "storage-check",
					Image: awsCliImage,
					// Verify the ephemeral storage mount is the requested 500Gi.
					Command: []string{
						"/bin/sh", "-c",
						`size=$(df --output=size --block-size=GiB /data | tail -1 | tr -d 'GiB '); [ "$size" -ge 500 ] && echo "ok: ${size}GiB" || { echo "fail: size is ${size}GiB"; exit 1; }`,
					},
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:              resource.MustParse("1m"),
							v1.ResourceMemory:           resource.MustParse("50Mi"),
							v1.ResourceEphemeralStorage: resource.MustParse("500Gi"),
						},
						Limits: v1.ResourceList{
							v1.ResourceCPU:              resource.MustParse("1m"),
							v1.ResourceMemory:           resource.MustParse("50Mi"),
							v1.ResourceEphemeralStorage: resource.MustParse("500Gi"),
						},
					},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "ephemeral",
							MountPath: "/data",
						},
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: "ephemeral",
					VolumeSource: v1.VolumeSource{
						EmptyDir: &v1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}
}
