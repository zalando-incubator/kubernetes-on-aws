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

	awsiamrole "github.com/zalando-incubator/kube-aws-iam-controller/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

var _ = framework.KubeDescribe("AWS IAM Integration (kube-aws-iam-controller)", func() {
	f := framework.NewDefaultFramework("aws-iam")
	var cs kubernetes.Interface
	var zcs awsiamrole.Interface

	BeforeEach(func() {
		cs = f.ClientSet

		By("Creating an awsiamrole client")
		config, err := framework.LoadConfig()
		Expect(err).NotTo(HaveOccurred())
		config.QPS = f.Options.ClientQPS
		config.Burst = f.Options.ClientBurst
		if f.Options.GroupVersion != nil {
			config.GroupVersion = f.Options.GroupVersion
		}
		zcs, err = awsiamrole.NewForConfig(config)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should get AWS IAM credentials [AWS-IAM] [Zalando]", func() {
		awsIAMRoleRS := "aws-iam-test"
		ns := f.Namespace.Name

		By("Creating a awscli POD in namespace " + ns)
		pod := createAWSIAMPod("aws-iam-", ns, E2ES3AWSIAMBucket())
		_, err := cs.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		// AWSIAMRole
		By("Creating AWSIAMRole " + awsIAMRoleRS + " in namespace " + ns)
		rs := createAWSIAMRole(awsIAMRoleRS, ns, E2EAWSIAMRole())
		defer func() {
			By("deleting the AWSIAMRole")
			defer GinkgoRecover()
			err2 := zcs.ZalandoV1().AWSIAMRoles(ns).Delete(context.TODO(), rs.Name, metav1.DeleteOptions{})
			Expect(err2).NotTo(HaveOccurred())
		}()
		_, err = zcs.ZalandoV1().AWSIAMRoles(ns).Create(context.TODO(), rs, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		framework.ExpectNoError(e2epod.WaitForPodSuccessInNamespace(f.ClientSet, pod.Name, pod.Namespace))
	})

	It("Should NOT get AWS IAM credentials [AWS-IAM] [Zalando]", func() {
		ns := f.Namespace.Name

		By("Creating a awscli POD in namespace " + ns)
		pod := createAWSCLIPod("aws-iam-", ns, []string{"s3", "ls", fmt.Sprintf("s3://%s", E2ES3AWSIAMBucket())})
		_, err := cs.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		framework.ExpectNoError(e2epod.WaitForPodTerminatedInNamespace(f.ClientSet, pod.Name, "", pod.Namespace))

		p, err := cs.CoreV1().Pods(ns).Get(context.TODO(), pod.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(p.Status.ContainerStatuses).NotTo(BeEmpty(), "No container statuses found")
		Expect(p.Status.ContainerStatuses[0].State.Terminated).NotTo(BeNil(), "Expected to find a terminated container")
		Expect(p.Status.ContainerStatuses[0].State.Terminated.ExitCode).To(BeEquivalentTo(255), "Expected the container to exit with an error status code")
	})
})
