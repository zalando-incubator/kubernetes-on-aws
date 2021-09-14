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
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

const (
	compliantImage    = "registry.opensource.zalan.do/teapot/skipper:v0.13.98"       // this image tag is compliant
	compliantImage2   = "registry.opensource.zalan.do/teapot/skipper:v0.13.97"       // this image tag is compliant as well
	nonCompliantImage = "registry.opensource.zalan.do/teapot/skipper-test:pr-1845-1" // this image tag is not compliant
)

var _ = framework.KubeDescribe("Image Policy Tests (Deployment)", func() {
	f := framework.NewDefaultFramework("image-policy-test-enabled")
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("Should create Deployment with compliant image [Image-Policy] [Compliant] [Zalando]", func() {
		namePrefix := "image-policy-test-compliant"
		appLabel := fmt.Sprintf("image-policy-test-pod-%s", uuid.NewUUID())
		namespace := f.Namespace.Name
		replicas := 2

		By("Creating Deployment " + namePrefix + " in namespace " + namespace)

		deployment := createImagePolicyWebhookTestDeployment(namePrefix, namespace, compliantImage, appLabel, int32(replicas))
		_, err := cs.AppsV1().Deployments(namespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		defer func() {
			By(fmt.Sprintf("Delete a Deployment: %s", deployment.Name))
			defer GinkgoRecover()
			err := cs.AppsV1().Deployments(namespace).Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}()

		err = waitForDeploymentWithCondition(cs, namespace, deployment.Name, "MinimumReplicasAvailable", appsv1.DeploymentAvailable)
		Expect(err).NotTo(HaveOccurred())

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), replicas, 1*time.Minute)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should not create Deployment using non-compliant image [Image-Policy] [Non-Compliant] [Zalando]", func() {
		namePrefix := "image-policy-test-non-compliant"
		podName := fmt.Sprintf("image-policy-test-pod-%s", uuid.NewUUID())
		namespace := f.Namespace.Name
		replicas := 1

		By("Creating Deployment " + namePrefix + " in namespace " + namespace)

		deployment := createImagePolicyWebhookTestDeployment(namePrefix, namespace, nonCompliantImage, podName, int32(replicas))
		_, err := cs.AppsV1().Deployments(namespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		defer func() {
			By(fmt.Sprintf("Delete a Deployment: %s", deployment.Name))
			defer GinkgoRecover()
			err := cs.AppsV1().Deployments(namespace).Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}()

		err = waitForDeploymentWithCondition(cs, namespace, deployment.Name, "FailedCreate", appsv1.DeploymentReplicaFailure)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = framework.KubeDescribe("Image Policy Tests (Deployment) (when disabled)", func() {
	f := framework.NewDefaultFramework("image-policy-test-disabled")
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("Should create Deployment using non-compliant image [Image-Policy] [Non-Compliant] [Zalando]", func() {
		namePrefix := "image-policy-test-non-compliant"
		appLabel := fmt.Sprintf("image-policy-test-pod-%s", uuid.NewUUID())
		replicas := 2
		namespace := f.Namespace.Name

		By("Creating Deployment " + namePrefix + " in namespace " + namespace)

		deployment := createImagePolicyWebhookTestDeployment(namePrefix, namespace, nonCompliantImage, appLabel, int32(replicas))
		_, err := cs.AppsV1().Deployments(namespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		defer func() {
			By(fmt.Sprintf("Delete a Deployment: %s", deployment.Name))
			defer GinkgoRecover()
			err := cs.AppsV1().Deployments(namespace).Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}()

		err = waitForDeploymentWithCondition(cs, namespace, deployment.Name, "MinimumReplicasAvailable", appsv1.DeploymentAvailable)
		Expect(err).NotTo(HaveOccurred())

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), replicas, 1*time.Minute)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = framework.KubeDescribe("Image Policy Tests (Pods)", func() {
	f := framework.NewDefaultFramework("image-policy-test-enabled")
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("Should create pod with compliant image [Image-Policy] [Compliant] [Zalando]", func() {
		namePrefix := "image-policy-test-compliant"
		appLabel := fmt.Sprintf("image-policy-test-pod-%s", uuid.NewUUID())
		namespace := f.Namespace.Name

		By("Creating pod " + namePrefix + " in namespace " + namespace)

		pod := createImagePolicyWebhookTestPod(namePrefix, namespace, compliantImage, appLabel)
		_, err := cs.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		defer func() {
			By(fmt.Sprintf("Delete a pod: %s", pod.Name))
			defer GinkgoRecover()
			err := cs.CoreV1().Pods(namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}()

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), 1, 1*time.Minute)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should not create pod with non-compliant image [Image-Policy] [Non-Compliant] [Zalando]", func() {
		namePrefix := "image-policy-test-non-compliant"
		podName := fmt.Sprintf("image-policy-test-pod-%s", uuid.NewUUID())
		namespace := f.Namespace.Name

		By("Creating pod " + namePrefix + " in namespace " + namespace)

		pod := createImagePolicyWebhookTestPod(namePrefix, namespace, nonCompliantImage, podName)
		_, err := cs.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
		Expect(err).To(HaveOccurred())
	})
})

var _ = framework.KubeDescribe("Image Policy Tests (Pods) (when disabled)", func() {
	f := framework.NewDefaultFramework("image-policy-test-disabled")
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("Should create pod with non-compliant image [Image-Policy] [Non-Compliant] [Zalando]", func() {
		namePrefix := "image-policy-test-non-compliant"
		appLabel := fmt.Sprintf("image-policy-test-pod-%s", uuid.NewUUID())
		namespace := f.Namespace.Name

		By("Creating pod " + namePrefix + " in namespace " + namespace)

		pod := createImagePolicyWebhookTestPod(namePrefix, namespace, nonCompliantImage, appLabel)
		_, err := cs.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		defer func() {
			By(fmt.Sprintf("Delete a pod: %s", pod.Name))
			defer GinkgoRecover()
			err := cs.CoreV1().Pods(namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}()

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), 1, 1*time.Minute)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = framework.KubeDescribe("Image Policy Tests (Pods Update Path)", func() {
	f := framework.NewDefaultFramework("image-policy-test-enabled")
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("Should update pod with compliant image [Image-Policy] [Compliant] [Zalando]", func() {
		namePrefix := "image-policy-test-compliant"
		appLabel := fmt.Sprintf("image-policy-test-pod-%s", uuid.NewUUID())
		namespace := f.Namespace.Name

		By("Creating pod " + namePrefix + " in namespace " + namespace)

		pod := createImagePolicyWebhookTestPod(namePrefix, namespace, compliantImage, appLabel)
		_, err := cs.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		defer func() {
			By(fmt.Sprintf("Delete a pod: %s", pod.Name))
			defer GinkgoRecover()
			err := cs.CoreV1().Pods(namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}()

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), 1, 1*time.Minute)
		Expect(err).NotTo(HaveOccurred())

		By("Updating pod " + namePrefix + " in namespace " + namespace)

		pod, err = cs.CoreV1().Pods(namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		pod.Spec.Containers[0].Image = compliantImage2

		_, err = cs.CoreV1().Pods(namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred())

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), 1, 1*time.Minute)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should not update pod with non-compliant image [Image-Policy] [Non-Compliant] [Zalando]", func() {
		namePrefix := "image-policy-test-compliant"
		appLabel := fmt.Sprintf("image-policy-test-pod-%s", uuid.NewUUID())
		namespace := f.Namespace.Name

		By("Creating pod " + namePrefix + " in namespace " + namespace)

		pod := createImagePolicyWebhookTestPod(namePrefix, namespace, compliantImage, appLabel)
		_, err := cs.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		defer func() {
			By(fmt.Sprintf("Delete a pod: %s", pod.Name))
			defer GinkgoRecover()
			err := cs.CoreV1().Pods(namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}()

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), 1, 1*time.Minute)
		Expect(err).NotTo(HaveOccurred())

		pod, err = cs.CoreV1().Pods(namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Updating pod " + namePrefix + " in namespace " + namespace)

		pod.Spec.Containers[0].Image = nonCompliantImage

		_, err = cs.CoreV1().Pods(namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})
		Expect(err).To(HaveOccurred())
	})
})

var _ = framework.KubeDescribe("Image Policy Tests (Pods Update Path) (when disabled)", func() {
	f := framework.NewDefaultFramework("image-policy-test-disabled")
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("Should update pod with non-compliant image [Image-Policy] [Non-Compliant] [Zalando]", func() {
		namePrefix := "image-policy-test-compliant"
		appLabel := fmt.Sprintf("image-policy-test-pod-%s", uuid.NewUUID())
		namespace := f.Namespace.Name

		By("Creating pod " + namePrefix + " in namespace " + namespace)

		pod := createImagePolicyWebhookTestPod(namePrefix, namespace, compliantImage, appLabel)
		_, err := cs.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		defer func() {
			By(fmt.Sprintf("Delete a pod: %s", pod.Name))
			defer GinkgoRecover()
			err := cs.CoreV1().Pods(namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}()

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), 1, 1*time.Minute)
		Expect(err).NotTo(HaveOccurred())

		pod, err = cs.CoreV1().Pods(namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Updating pod " + namePrefix + " in namespace " + namespace)

		pod.Spec.Containers[0].Image = nonCompliantImage

		_, err = cs.CoreV1().Pods(namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred())

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), 1, 1*time.Minute)
		Expect(err).NotTo(HaveOccurred())
	})
})
