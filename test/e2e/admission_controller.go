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
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	deploymentId = "d-2ajbkvmtqo5isznh3m2raj2bdp"
	pipelineId   = "l-2kwqgqevuqwsje5a9ukhsk4bdd"
)

var _ = framework.KubeDescribe("Admission controller tests", func() {
	f := framework.NewDefaultFramework("zalando-kube-admission-controller")
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("Pods should get deployment info from deployment and zone from node [Zalando]", func() {
		nameprefix := "deployment-info-test"
		podname := fmt.Sprintf("deployment-info-test-pod")
		var replicas int32 = 2
		ns := f.Namespace.Name

		By("Creating deployment " + nameprefix + " in namespace " + ns)

		deployment := createDeploymentWithDeploymentInfo(nameprefix+"-", ns, podname, replicas)
		_, err := cs.ExtensionsV1beta1().Deployments(ns).Create(deployment)
		Expect(err).NotTo(HaveOccurred())
		label := map[string]string{
			"app": podname,
		}
		labelSelector := labels.SelectorFromSet(labels.Set(label))
		err = framework.WaitForDeploymentWithCondition(cs, ns, deployment.Name, "MinimumReplicasAvailable", appsv1.DeploymentAvailable)
		Expect(err).NotTo(HaveOccurred())

		//pods are not returned here
		_, err = framework.WaitForPodsWithLabelRunningReady(cs, ns, labelSelector, int(replicas), 1*time.Minute)
		Expect(err).NotTo(HaveOccurred())

		pods, err := cs.CoreV1().Pods(ns).List(metav1.ListOptions{LabelSelector: labelSelector.String()})
		Expect(err).NotTo(HaveOccurred())

		Expect(len(pods.Items)).To(Equal(2))

		pod := pods.Items[0]
		Expect(pod.Annotations).To(HaveKeyWithValue("zalando.org/cdp-deployment-id", deploymentId))
		Expect(pod.Annotations).To(HaveKeyWithValue("zalando.org/cdp-pipeline-id", pipelineId))

		zone := pod.Annotations["failure-domain.beta.kubernetes.io/zone"]
		Expect(zone).To(HavePrefix("eu-central-"))

		platformEnvVar := 0
		for _, v := range pod.Spec.Containers[0].Env {
			if strings.HasPrefix(v.Name, "_PLATFORM_") {
				platformEnvVar++
			}
		}

		Expect(platformEnvVar >= 14).To(BeTrue())

		bytes, err := cs.CoreV1().Pods(ns).GetLogs(pod.Name, &v1.PodLogOptions{}).DoRaw()
		lines := strings.Split(string(bytes), "\n")
		Expect(lines).To(ContainElement(deploymentId))
		Expect(lines).To(ContainElement(pipelineId))
		Expect(lines).To(ContainElement(zone))
	})

})

func createDeploymentWithDeploymentInfo(nameprefix, namespace, podname string, replicas int32) *v1beta1.Deployment {
	zero := int64(0)
	return &v1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameprefix + string(uuid.NewUUID()),
			Namespace: namespace,
			Labels: map[string]string{
				"deployment-id": deploymentId,
				"pipeline-id":   pipelineId,
			},
		},
		Spec: v1beta1.DeploymentSpec{
			Replicas: &replicas,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": podname,
					},
				},
				Spec: v1.PodSpec{
					TerminationGracePeriodSeconds: &zero,
					Containers: []v1.Container{
						{
							Name:    "admission-controller-test",
							Image:   "k8s.gcr.io/busybox",
							Command: []string{"sh", "-c"},
							Args: []string{
								` while true; do
									printenv _PLATFORM_ZONE;
									printenv _PLATFORM_DEPLOYMENT_ID;
									printenv _PLATFORM_PIPELINE_ID;
									sleep 1000;
								  done;`,
							},
						},
					},
				},
			},
		},
	}
}
