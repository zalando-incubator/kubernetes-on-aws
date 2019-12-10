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
this component is purposed to tests webhooks in the apiserver
*/

package e2e

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	deploymentframework "k8s.io/kubernetes/test/e2e/framework/deployment"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

var _ = framework.KubeDescribe("API Server webhook tests", func() {
	f := framework.NewDefaultFramework("zalando-kube-api-server")
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("Should create deployment with compliant image [Image-Webhook] [Compliant] [Zalando]", func() {
		tag := "bc1a6fe"

		nameprefix := "image-policy-webhook-test-compliant"
		podname := fmt.Sprintf("image-webhook-policy-test-pod-%s", tag)
		var replicas int32 = 2
		ns := f.Namespace.Name

		By("Creating deployment " + nameprefix + " in namespace " + ns)

		deployment := createImagePolicyWebhookTestDeployment(nameprefix+"-", ns, tag, podname, replicas)
		_, err := cs.AppsV1().Deployments(ns).Create(deployment)
		defer func() {
			By(fmt.Sprintf("Delete a compliant deployment: %s", deployment.Name))
			defer GinkgoRecover()
			err := cs.AppsV1().Deployments(ns).Delete(deployment.Name, metav1.NewDeleteOptions(0))
			Expect(err).NotTo(HaveOccurred())
		}()
		Expect(err).NotTo(HaveOccurred())
		label := map[string]string{
			"app": podname,
		}
		labelSelector := labels.SelectorFromSet(labels.Set(label))
		err = deploymentframework.WaitForDeploymentWithCondition(cs, ns, deployment.Name, "MinimumReplicasAvailable", appsv1.DeploymentAvailable)
		Expect(err).NotTo(HaveOccurred())
		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, ns, labelSelector, int(replicas), 1*time.Minute)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should not allow deployment using not trusted image [Image-Webhook] [Non-Compliant] [Zalando]", func() {
		tag := "bc1a6fe-nottrusted2"

		nameprefix := "image-policy-webhook-test-non-compliant"
		podname := fmt.Sprintf("image-webhook-policy-test-pod-%s", tag)
		var replicas int32 = 1
		ns := f.Namespace.Name

		By("Creating deployment " + nameprefix + " in namespace " + ns)

		deployment := createImagePolicyWebhookTestDeployment(nameprefix+"-", ns, tag, podname, replicas)
		_, err := cs.AppsV1().Deployments(ns).Create(deployment)
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			By(fmt.Sprintf("Delete a compliant deployment: %s", deployment.Name))
			defer GinkgoRecover()
			err := cs.AppsV1().Deployments(ns).Delete(deployment.Name, metav1.NewDeleteOptions(0))
			Expect(err).NotTo(HaveOccurred())
		}()
		err = deploymentframework.WaitForDeploymentWithCondition(cs, ns, deployment.Name, "FailedCreate", appsv1.DeploymentReplicaFailure)
		Expect(err).NotTo(HaveOccurred())
	})
})
