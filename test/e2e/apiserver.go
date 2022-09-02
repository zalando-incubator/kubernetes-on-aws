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
	"k8s.io/kubernetes/test/e2e/framework/job"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/kubernetes/test/e2e/framework/statefulset"
)

const (
	compliantImage    = "registry.opensource.zalan.do/teapot/skipper:v0.13.98"       // this image tag is compliant
	compliantImage2   = "registry.opensource.zalan.do/teapot/skipper:v0.13.97"       // this image tag is compliant as well
	nonCompliantImage = "registry.opensource.zalan.do/teapot/skipper-test:pr-1845-1" // this image tag is not compliant
	waitForPodTimeout = 5 * time.Minute
)

var _ = describe("Image Policy Tests (Deployment)", func() {
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

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), replicas, waitForPodTimeout)
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

var _ = describe("Image Policy Tests (Deployment) (when disabled)", func() {
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

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), replicas, waitForPodTimeout)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = describe("Image Policy Tests (Pods)", func() {
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

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), 1, waitForPodTimeout)
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

var _ = describe("Image Policy Tests (Pods) (when disabled)", func() {
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

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), 1, waitForPodTimeout)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = describe("Image Policy Tests (Pods Update Path)", func() {
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

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), 1, waitForPodTimeout)
		Expect(err).NotTo(HaveOccurred())

		By("Updating pod " + namePrefix + " in namespace " + namespace)

		pod, err = cs.CoreV1().Pods(namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		pod.Spec.Containers[0].Image = compliantImage2

		_, err = cs.CoreV1().Pods(namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred())

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), 1, waitForPodTimeout)
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

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), 1, waitForPodTimeout)
		Expect(err).NotTo(HaveOccurred())

		pod, err = cs.CoreV1().Pods(namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Updating pod " + namePrefix + " in namespace " + namespace)

		pod.Spec.Containers[0].Image = nonCompliantImage

		_, err = cs.CoreV1().Pods(namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})
		Expect(err).To(HaveOccurred())
	})
})

var _ = describe("Image Policy Tests (Pods Update Path) (when disabled)", func() {
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

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), 1, waitForPodTimeout)
		Expect(err).NotTo(HaveOccurred())

		pod, err = cs.CoreV1().Pods(namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Updating pod " + namePrefix + " in namespace " + namespace)

		pod.Spec.Containers[0].Image = nonCompliantImage

		_, err = cs.CoreV1().Pods(namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred())

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), 1, waitForPodTimeout)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = describe("Image Policy Tests (StatefulSet)", func() {
	f := framework.NewDefaultFramework("image-policy-test-enabled")
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("Should create StatefulSet with compliant image [Image-Policy] [Compliant] [Zalando]", func() {
		namePrefix := "ip-compliant"
		appLabel := fmt.Sprintf("image-policy-test-pod-%s", uuid.NewUUID())
		namespace := f.Namespace.Name
		replicas := 2

		By("Creating StatefulSet " + namePrefix + " in namespace " + namespace)

		statefulSet := createImagePolicyWebhookTestStatefulSet(namePrefix, namespace, compliantImage, appLabel, int32(replicas))
		_, err := cs.AppsV1().StatefulSets(namespace).Create(context.TODO(), statefulSet, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		defer func() {
			By(fmt.Sprintf("Delete a StatefulSet: %s", statefulSet.Name))
			defer GinkgoRecover()
			err := cs.AppsV1().StatefulSets(namespace).Delete(context.TODO(), statefulSet.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}()

		statefulset.WaitForRunningAndReady(cs, int32(replicas), statefulSet)

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), replicas, waitForPodTimeout)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should not create StatefulSet using non-compliant image [Image-Policy] [Non-Compliant] [Zalando]", func() {
		namePrefix := "ip-noncompliant"
		appLabel := fmt.Sprintf("image-policy-test-pod-%s", uuid.NewUUID())
		namespace := f.Namespace.Name
		replicas := 1

		By("Creating StatefulSet " + namePrefix + " in namespace " + namespace)

		statefulSet := createImagePolicyWebhookTestStatefulSet(namePrefix, namespace, nonCompliantImage, appLabel, int32(replicas))
		_, err := cs.AppsV1().StatefulSets(namespace).Create(context.TODO(), statefulSet, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		defer func() {
			By(fmt.Sprintf("Delete a StatefulSet: %s", statefulSet.Name))
			defer GinkgoRecover()
			err := cs.AppsV1().StatefulSets(namespace).Delete(context.TODO(), statefulSet.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}()

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), 1, 1*time.Minute)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(MatchRegexp("Timeout while waiting for pods with label application=%s", appLabel))
	})
})

var _ = describe("Image Policy Tests (StatefulSet) (when disabled)", func() {
	f := framework.NewDefaultFramework("image-policy-test-disabled")
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("Should create StatefulSet using non-compliant image [Image-Policy] [Non-Compliant] [Zalando]", func() {
		namePrefix := "ip-noncompliant"
		appLabel := fmt.Sprintf("image-policy-test-pod-%s", uuid.NewUUID())
		replicas := 2
		namespace := f.Namespace.Name

		By("Creating StatefulSet " + namePrefix + " in namespace " + namespace)

		statefulSet := createImagePolicyWebhookTestStatefulSet(namePrefix, namespace, nonCompliantImage, appLabel, int32(replicas))
		_, err := cs.AppsV1().StatefulSets(namespace).Create(context.TODO(), statefulSet, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		defer func() {
			By(fmt.Sprintf("Delete a StatefulSet: %s", statefulSet.Name))
			defer GinkgoRecover()
			err := cs.AppsV1().StatefulSets(namespace).Delete(context.TODO(), statefulSet.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}()

		statefulset.WaitForRunningAndReady(cs, int32(replicas), statefulSet)

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), replicas, waitForPodTimeout)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = describe("Image Policy Tests (Job)", func() {
	f := framework.NewDefaultFramework("image-policy-test-enabled")
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("Should create Job with compliant image [Image-Policy] [Compliant] [Zalando]", func() {
		namePrefix := "ipt-compliant"
		appLabel := fmt.Sprintf("image-policy-test-pod-%s", uuid.NewUUID())
		namespace := f.Namespace.Name

		By("Creating Job " + namePrefix + " in namespace " + namespace)

		jobObj := createImagePolicyWebhookTestJob(namePrefix, namespace, compliantImage, appLabel)
		_, err := cs.BatchV1().Jobs(namespace).Create(context.TODO(), jobObj, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		defer func() {
			By(fmt.Sprintf("Delete a Job: %s", jobObj.Name))
			defer GinkgoRecover()
			err := cs.BatchV1().Jobs(namespace).Delete(context.TODO(), jobObj.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}()

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), 1, waitForPodTimeout)
		Expect(err).NotTo(HaveOccurred())

		job.WaitForJobFinish(cs, namespace, jobObj.Name)
	})

	It("Should not create Job using non-compliant image [Image-Policy] [Non-Compliant] [Zalando]", func() {
		namePrefix := "ipt-non-compliant"
		appLabel := fmt.Sprintf("image-policy-test-pod-%s", uuid.NewUUID())
		namespace := f.Namespace.Name

		By("Creating Job " + namePrefix + " in namespace " + namespace)

		jobObj := createImagePolicyWebhookTestJob(namePrefix, namespace, nonCompliantImage, appLabel)
		_, err := cs.BatchV1().Jobs(namespace).Create(context.TODO(), jobObj, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		defer func() {
			By(fmt.Sprintf("Delete a Job: %s", jobObj.Name))
			defer GinkgoRecover()
			err := cs.BatchV1().Jobs(namespace).Delete(context.TODO(), jobObj.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}()

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), 1, 1*time.Minute)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(MatchRegexp("Timeout while waiting for pods with label application=%s", appLabel))
	})
})

var _ = describe("Image Policy Tests (Job) (when disabled)", func() {
	f := framework.NewDefaultFramework("image-policy-test-disabled")
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("Should create Job using non-compliant image [Image-Policy] [Non-Compliant] [Zalando]", func() {
		namePrefix := "ip-noncompliant"
		appLabel := fmt.Sprintf("image-policy-test-pod-%s", uuid.NewUUID())
		namespace := f.Namespace.Name

		By("Creating Job " + namePrefix + " in namespace " + namespace)

		jobObj := createImagePolicyWebhookTestJob(namePrefix, namespace, nonCompliantImage, appLabel)
		_, err := cs.BatchV1().Jobs(namespace).Create(context.TODO(), jobObj, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		defer func() {
			By(fmt.Sprintf("Delete a Job: %s", jobObj.Name))
			defer GinkgoRecover()
			err := cs.BatchV1().Jobs(namespace).Delete(context.TODO(), jobObj.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}()

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), 1, waitForPodTimeout)
		Expect(err).NotTo(HaveOccurred())

		job.WaitForJobFinish(cs, namespace, jobObj.Name)
	})
})

var _ = describe("ECR Registry Pull", func() {
	f := framework.NewDefaultFramework("ecr-registry")
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("Should run a Job using an image from Staging ECR [ECR] [Zalando]", func() {
		namePrefix := "ecr-registry-test"
		appLabel := fmt.Sprintf("ecr-image-pull-staging-%s", uuid.NewUUID())
		namespace := f.Namespace.Name

		ecrStagingImage := "926694233939.dkr.ecr.eu-central-1.amazonaws.com/staging_namespace/automata/busybox:uno"
		args := []string{"sleep", "10"}

		By("Creating Job " + namePrefix + " in namespace " + namespace)

		jobObj := createTestJob(namePrefix, "ecr-image-pull-test", namespace, ecrStagingImage, appLabel, args)
		_, err := cs.BatchV1().Jobs(namespace).Create(context.TODO(), jobObj, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		defer func() {
			By(fmt.Sprintf("Delete a Job: %s", jobObj.Name))
			defer GinkgoRecover()
			err := cs.BatchV1().Jobs(namespace).Delete(context.TODO(), jobObj.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}()

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), 1, waitForPodTimeout)
		Expect(err).NotTo(HaveOccurred())

		job.WaitForJobFinish(cs, namespace, jobObj.Name)
	})

	It("Should run a Job using a vanity image from the staging registry [ECR] [Zalando]", func() {
		namePrefix := "ecr-registry-test"
		appLabel := fmt.Sprintf("ecr-image-pull-staging-%s", uuid.NewUUID())
		namespace := f.Namespace.Name

		vanityStagingImage := "container-registry-test.zalando.net/automata/busybox:uno"
		args := []string{"sleep", "10"}

		By("Creating Job " + namePrefix + " in namespace " + namespace)

		jobObj := createTestJob(namePrefix, "ecr-image-pull-test", namespace, vanityStagingImage, appLabel, args)
		_, err := cs.BatchV1().Jobs(namespace).Create(context.TODO(), jobObj, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		defer func() {
			By(fmt.Sprintf("Delete a Job: %s", jobObj.Name))
			defer GinkgoRecover()
			err := cs.BatchV1().Jobs(namespace).Delete(context.TODO(), jobObj.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}()

		_, err = e2epod.WaitForPodsWithLabelRunningReady(cs, namespace, appLabelSelector(appLabel), 1, waitForPodTimeout)
		Expect(err).NotTo(HaveOccurred())

		job.WaitForJobFinish(cs, namespace, jobObj.Name)
	})
})
