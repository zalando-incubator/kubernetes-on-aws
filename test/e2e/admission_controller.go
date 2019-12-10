/*
Copyright 2015 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/**
this component is purposed to tests webhooks in the admission controller
*/

package e2e

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	deploymentframework "k8s.io/kubernetes/test/e2e/framework/deployment"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

const (
	deploymentId = "d-2ajbkvmtqo5isznh3m2raj2bdp"
	pipelineId   = "l-2kwqgqevuqwsje5a9ukhsk4bdd"
	application  = "e2e-test-application"
	component    = "e2e-test-component"
	environment  = "e2e-test-environment"
	dockerImage  = "k8s.gcr.io/busybox"
)

var _ = framework.KubeDescribe("Admission controller tests", func() {
	f := framework.NewDefaultFramework("zalando-kube-admission-controller")
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("Admission controller should inject platform environment variables [Zalando]", func() {
		nameprefix := "deployment-info-test"
		podname := fmt.Sprintf("deployment-info-test-pod")
		var replicas int32 = 1
		ns := f.Namespace.Name

		By("Creating deployment " + nameprefix + " in namespace " + ns)

		deployment := createDeploymentWithDeploymentInfo(nameprefix+"-", ns, podname, replicas)
		_, err := cs.AppsV1().Deployments(ns).Create(deployment)
		Expect(err).NotTo(HaveOccurred())
		labelSelector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
		Expect(err).NotTo(HaveOccurred())
		err = deploymentframework.WaitForDeploymentWithCondition(cs, ns, deployment.Name, "MinimumReplicasAvailable", appsv1.DeploymentAvailable)
		Expect(err).NotTo(HaveOccurred())

		//pods are not returned here
		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, ns, labelSelector, int(replicas), 1*time.Minute)
		Expect(err).NotTo(HaveOccurred())

		pods, err := cs.CoreV1().Pods(ns).List(metav1.ListOptions{LabelSelector: labelSelector.String()})
		Expect(err).NotTo(HaveOccurred())
		Expect(len(pods.Items)).To(Equal(1))

		pod := pods.Items[0]
		Expect(pod.Annotations).To(HaveKeyWithValue("zalando.org/cdp-deployment-id", deploymentId))
		Expect(pod.Annotations).To(HaveKeyWithValue("zalando.org/cdp-pipeline-id", pipelineId))

		// Check the injected node zone
		node, err := cs.CoreV1().Nodes().Get(pod.Spec.NodeName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		nodeZone := node.Labels["failure-domain.beta.kubernetes.io/zone"]
		Expect(pod.Annotations).To(HaveKeyWithValue("failure-domain.beta.kubernetes.io/zone", nodeZone))

		envarValues, err := fetchEnvarValues(cs, ns, pod.Name)
		Expect(err).NotTo(HaveOccurred())

		// Check the environment variable values

		// Static
		Expect(envarValues).To(HaveKeyWithValue("_PLATFORM_ACCOUNT", E2EClusterAlias()))
		Expect(envarValues).To(HaveKeyWithValue("_PLATFORM_CLUSTER_ID", E2EClusterID()))
		Expect(envarValues).To(HaveKeyWithValue("_PLATFORM_OPENTRACING_TAG_ACCOUNT", E2EClusterAlias()))
		Expect(envarValues).To(HaveKeyWithValue("_PLATFORM_OPENTRACING_TAG_APPLICATION", application))
		Expect(envarValues).To(HaveKeyWithValue("_PLATFORM_OPENTRACING_LIGHTSTEP_COLLECTOR_PORT", Not(BeEmpty())))
		Expect(envarValues).To(HaveKeyWithValue("_PLATFORM_OPENTRACING_LIGHTSTEP_COLLECTOR_HOST", Not(BeEmpty())))
		Expect(envarValues).To(HaveKeyWithValue("_PLATFORM_OPENTRACING_LIGHTSTEP_ACCESS_TOKEN", Not(BeEmpty())))

		// Dynamic
		Expect(envarValues).To(HaveKeyWithValue("_PLATFORM_APPLICATION", application))
		Expect(envarValues).To(HaveKeyWithValue("_PLATFORM_COMPONENT", component))
		Expect(envarValues).To(HaveKeyWithValue("_PLATFORM_ENVIRONMENT", environment))
		Expect(envarValues).To(HaveKeyWithValue("_PLATFORM_DEPLOYMENT_ID", deploymentId))
		Expect(envarValues).To(HaveKeyWithValue("_PLATFORM_OPENTRACING_TAG_DEPLOYMENT_ID", deploymentId))
		Expect(envarValues).To(HaveKeyWithValue("_PLATFORM_PIPELINE_ID", pipelineId))
		Expect(envarValues).To(HaveKeyWithValue("_PLATFORM_ZONE", nodeZone))
		Expect(envarValues).To(HaveKeyWithValue("_PLATFORM_OPENTRACING_TAG_ZONE", nodeZone))
		Expect(envarValues).To(HaveKeyWithValue("_PLATFORM_DOCKER_IMAGE", dockerImage))
		Expect(envarValues).To(HaveKeyWithValue("_PLATFORM_OPENTRACING_TAG_ARTIFACT", dockerImage))

		// User-set
		Expect(envarValues).To(HaveKeyWithValue("_PLATFORM_E2E", "overridden"))
	})

	It("Admission controller should not prevent pods from being scheduled [Zalando]", func() {
		ns := f.Namespace.Name
		podName := "admission-invalid-owner-" + string(uuid.NewUUID())

		By("Creating pod " + podName + " in namespace " + ns)
		pod := createInvalidOwnerPod(ns, podName)
		_, err := cs.CoreV1().Pods(ns).Create(pod)
		Expect(err).NotTo(HaveOccurred())

		err = e2epod.WaitForPodSuccessInNamespaceSlow(cs, podName, ns)
		Expect(err).NotTo(HaveOccurred())
	})
})

func fetchEnvarValues(client kubernetes.Interface, ns, pod string) (map[string]string, error) {
	result := make(map[string]string)

	bytes, err := client.CoreV1().Pods(ns).GetLogs(pod, &v1.PodLogOptions{}).DoRaw()
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(bytes), "\n") {
		kv := strings.SplitN(line, "=", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}
	return result, nil
}

func createInvalidOwnerPod(namespace, podname string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podname,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "foo/v1",
					Kind:       "Invalid",
					Name:       "asd",
					UID:        "abc-213-def",
				},
			},
		},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyOnFailure,
			Containers: []v1.Container{
				{
					Name:    "admission-controller-test",
					Image:   dockerImage,
					Command: []string{"/bin/true"},
				},
			},
		},
	}
}

func createDeploymentWithDeploymentInfo(nameprefix, namespace, podname string, replicas int32) *appsv1.Deployment {
	zero := int64(0)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameprefix + string(uuid.NewUUID()),
			Namespace: namespace,
			Labels: map[string]string{
				"deployment-id": deploymentId,
				"pipeline-id":   pipelineId,
				"application":   application,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"application": application}},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"application": application,
						"component":   component,
						"environment": environment,
					},
				},
				Spec: v1.PodSpec{
					TerminationGracePeriodSeconds: &zero,
					Containers: []v1.Container{
						{
							Name:    "admission-controller-test",
							Image:   dockerImage,
							Command: []string{"sh", "-c"},
							Args: []string{
								`env && sleep 100000`,
							},
							Env: []v1.EnvVar{
								{
									Name:  "_PLATFORM_E2E",
									Value: "overridden",
								},
							},
						},
					},
				},
			},
		},
	}
}
