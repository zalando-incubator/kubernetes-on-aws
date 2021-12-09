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

package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	deploymentutil "k8s.io/kubernetes/pkg/controller/deployment/util"
	"k8s.io/kubernetes/test/e2e/framework"
	deploymentframework "k8s.io/kubernetes/test/e2e/framework/deployment"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

var _ = describe("PSP use", func() {
	privilegedRole := "privileged-psp"
	privilegedSA := "privileged-sa"
	f := framework.NewDefaultFramework("psp")
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
		saObj := createServiceAccount(f.Namespace.Name, privilegedSA)
		_, err := cs.CoreV1().ServiceAccounts(f.Namespace.Name).Create(context.TODO(), saObj, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		_, err = cs.RbacV1().RoleBindings(f.Namespace.Name).Create(context.TODO(), createRBACRoleBindingSA(privilegedRole, f.Namespace.Name, privilegedSA), metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
	})

	// TODO: We have to have a solution to get an unprivileged
	// User to check this, if not it would always create a
	// privileged POD for an unprivileged serviceAccount.
	// --
	// It("Should not create a POD that use privileged PSP [PSP] [Zalando]", func() {
	//      defaultSA := "default"
	// 	ns := f.Namespace.Name
	// 	label := map[string]string{
	// 		"app": "psp",
	// 	}
	// 	msg := fmt.Sprintf("Creating a privileged POD as %s", defaultSA)
	// 	By(msg)
	//      route := fmt.Sprintf(`* -> inlineContent("%s") -> <shunt>`, "OK")
	// 	pod := createSkipperPodWithHostNetwork("", ns, defaultSA, route, label, 80)
	// 	defer func() {
	// 		By(msg)
	// 		defer GinkgoRecover()
	// 		err := cs.CoreV1().Pods(ns).Delete(pod.Name, metav1.NewDeleteOptions(0))
	// 		Expect(err).To(HaveOccurred())
	// 	}()
	// 	_, err := cs.CoreV1().Pods(ns).Create(pod)
	// 	Expect(err).To(HaveOccurred())
	// 	framework.ExpectNoError(f.WaitForPodRunning(pod.Name))
	// })

	It("Should create a POD that use privileged PSP [PSP] [Zalando]", func() {
		ns := f.Namespace.Name
		label := map[string]string{
			"app": "psp",
		}
		port := 81
		msg := fmt.Sprintf("Creating a privileged POD as %s", privilegedSA)

		By(msg)
		route := fmt.Sprintf(`* -> inlineContent("%s") -> <shunt>`, "OK")
		pod := createSkipperPodWithHostNetwork("", ns, privilegedSA, route, label, port)
		defer func() {
			By(msg)
			defer GinkgoRecover()
			err := cs.CoreV1().Pods(ns).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}()

		_, err := cs.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		framework.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(f.ClientSet, pod.Name, pod.Namespace))
	})

	It("Should create a POD that use privileged PSP via deployment [PSP] [Zalando]", func() {
		ns := f.Namespace.Name
		label := map[string]string{
			"app": "psp",
		}
		labelSelector := labels.SelectorFromSet(labels.Set(label))

		replicas := int32(1)
		port := int32(82)

		By(fmt.Sprintf("Creating a deployment that creates a privileged POD as %s", privilegedSA))
		route := fmt.Sprintf(`* -> inlineContent("%s") -> <shunt>`, "OK")
		d := createSkipperBackendDeploymentWithHostNetwork("psp-test-", ns, privilegedSA, route, label, port, replicas)
		d.Annotations = map[string]string{"test": "should-copy-to-replica-set", v1.LastAppliedConfigAnnotation: "should-not-copy-to-replica-set"}

		defer func() {
			By(fmt.Sprintf("Delete a deployment that creates a privileged POD as %s", privilegedSA))
			defer GinkgoRecover()
			err := cs.AppsV1().Deployments(ns).Delete(context.TODO(), d.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}()

		deploy, err := cs.AppsV1().Deployments(ns).Create(context.TODO(), d, metav1.CreateOptions{})

		Expect(err).NotTo(HaveOccurred())

		// Wait for it to be updated to revision 1
		err = deploymentframework.WaitForDeploymentRevisionAndImage(cs, ns, deploy.Name, "1", d.Spec.Template.Spec.Containers[0].Image)
		Expect(err).NotTo(HaveOccurred())
		err = deploymentframework.WaitForDeploymentComplete(cs, deploy)
		Expect(err).NotTo(HaveOccurred())
		deployment, err := cs.AppsV1().Deployments(ns).Get(context.TODO(), deploy.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		rs, err := deploymentutil.GetNewReplicaSet(deployment, cs.AppsV1())
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("Got rs: %s, from deployment: %s", rs.Name, deploy.Name))

		pods, err := e2epod.PodsCreatedByLabel(f.ClientSet, f.Namespace.Name, rs.Name, replicas, labelSelector)
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("Ensuring each pod is running for rs: %s, pod: %s", rs.Name, pods.Items[0].Name))
		// Wait for the pods to enter the running state. Waiting loops until the pods
		// are running so non-running pods cause a timeout for this test.
		for _, pod := range pods.Items {
			if pod.DeletionTimestamp != nil {
				continue
			}
			err = e2epod.WaitForPodNameRunningInNamespace(f.ClientSet, pod.Name, pod.Namespace)
			Expect(err).NotTo(HaveOccurred())
		}
	})
})
