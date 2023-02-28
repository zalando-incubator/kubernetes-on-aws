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
	compliantImage1     = "container-registry.zalando.net/teapot/skipper:v0.15.33" // these are several compliant images
	compliantImage2     = "container-registry.zalando.net/teapot/skipper:v0.15.32"
	compliantImage3     = "container-registry.zalando.net/teapot/skipper:v0.15.31"
	compliantImage4     = "container-registry.zalando.net/teapot/skipper:v0.15.30"
	compliantImage5     = "container-registry.zalando.net/teapot/skipper:v0.15.29"
	compliantImage6     = "container-registry.zalando.net/teapot/skipper:v0.15.28"
	compliantImage7     = "container-registry.zalando.net/teapot/skipper:v0.15.27"
	compliantImage8     = "container-registry.zalando.net/teapot/skipper:v0.15.26"
	nonCompliantImage1  = "container-registry-test.zalando.net/teapot/skipper-test:pr-2216-11" // these are several non-compliant images
	nonCompliantImage2  = "container-registry-test.zalando.net/teapot/skipper-test:pr-2237-10"
	nonCompliantImage3  = "container-registry-test.zalando.net/teapot/skipper-test:pr-2238-2"
	nonCompliantImage4  = "container-registry-test.zalando.net/teapot/skipper-test:pr-2238-3"
	nonCompliantImage5  = "container-registry-test.zalando.net/teapot/skipper-test:pr-2239-1"
	nonCompliantImage6  = "container-registry-test.zalando.net/teapot/skipper-test:pr-2241-1"
	nonCompliantImage7  = "container-registry-test.zalando.net/teapot/skipper-test:pr-2241-3"
	nonCompliantImage8  = "container-registry-test.zalando.net/teapot/skipper-test:pr-2242-1"
	nonCompliantImage9  = "container-registry-test.zalando.net/teapot/skipper-test:pr-2242-5"
	nonCompliantImage10 = "container-registry-test.zalando.net/teapot/skipper-test:pr-2244-2"
	waitForPodTimeout   = 5 * time.Minute
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

		deployment := createImagePolicyWebhookTestDeployment(namePrefix, namespace, compliantImage1, appLabel, int32(replicas))
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

		deployment := createImagePolicyWebhookTestDeployment(namePrefix, namespace, nonCompliantImage1, podName, int32(replicas))
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

		deployment := createImagePolicyWebhookTestDeployment(namePrefix, namespace, nonCompliantImage2, appLabel, int32(replicas))
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

		pod := createImagePolicyWebhookTestPod(namePrefix, namespace, compliantImage2, appLabel)
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

		pod := createImagePolicyWebhookTestPod(namePrefix, namespace, nonCompliantImage3, podName)
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

		pod := createImagePolicyWebhookTestPod(namePrefix, namespace, nonCompliantImage4, appLabel)
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

		pod := createImagePolicyWebhookTestPod(namePrefix, namespace, compliantImage3, appLabel)
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

		pod.Spec.Containers[0].Image = compliantImage4

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

		pod := createImagePolicyWebhookTestPod(namePrefix, namespace, compliantImage5, appLabel)
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

		pod.Spec.Containers[0].Image = nonCompliantImage5

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

		pod := createImagePolicyWebhookTestPod(namePrefix, namespace, compliantImage6, appLabel)
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

		pod.Spec.Containers[0].Image = nonCompliantImage6

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

		statefulSet := createImagePolicyWebhookTestStatefulSet(namePrefix, namespace, compliantImage7, appLabel, int32(replicas))
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

		statefulSet := createImagePolicyWebhookTestStatefulSet(namePrefix, namespace, nonCompliantImage7, appLabel, int32(replicas))
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

		statefulSet := createImagePolicyWebhookTestStatefulSet(namePrefix, namespace, nonCompliantImage8, appLabel, int32(replicas))
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

		jobObj := createImagePolicyWebhookTestJob(namePrefix, namespace, compliantImage8, appLabel)
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

		jobObj := createImagePolicyWebhookTestJob(namePrefix, namespace, nonCompliantImage9, appLabel)
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

		jobObj := createImagePolicyWebhookTestJob(namePrefix, namespace, nonCompliantImage10, appLabel)
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

		ecrStagingImage := "926694233939.dkr.ecr.eu-central-1.amazonaws.com/staging_namespace/library/alpine-3:3-20230102"
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

		vanityStagingImage := "container-registry-test.zalando.net/library/alpine-3:3-20230102"
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
