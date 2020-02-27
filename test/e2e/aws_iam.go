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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	awsiamrole "github.com/zalando-incubator/kube-aws-iam-controller/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
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
		_, err := cs.CoreV1().Pods(ns).Create(pod)
		Expect(err).NotTo(HaveOccurred())

		// AWSIAMRole
		By("Creating AWSIAMRole " + awsIAMRoleRS + " in namespace " + ns)
		rs := createAWSIAMRole(awsIAMRoleRS, ns, E2EAWSIAMRole())
		defer func() {
			By("deleting the AWSIAMRole")
			defer GinkgoRecover()
			err2 := zcs.ZalandoV1().AWSIAMRoles(ns).Delete(rs.Name, metav1.NewDeleteOptions(0))
			Expect(err2).NotTo(HaveOccurred())
		}()
		_, err = zcs.ZalandoV1().AWSIAMRoles(ns).Create(rs)
		Expect(err).NotTo(HaveOccurred())

		framework.ExpectNoError(f.WaitForPodRunning(pod.Name))

		// wait for pod to access s3 and POD exit code 0
		for {
			p, err := cs.CoreV1().Pods(ns).Get(pod.Name, metav1.GetOptions{})
			if err != nil {
				Expect(fmt.Errorf("Could not get POD %s", pod.Name)).NotTo(HaveOccurred())
				return
			}

			if p.Status.ContainerStatuses[0].State.Terminated == nil {
				time.Sleep(10 * time.Second)
				continue
			}

			n := p.Status.ContainerStatuses[0].State.Terminated.ExitCode
			if n > 0 {
				// set error
				Expect(fmt.Errorf("failed to access s3 bucket")).NotTo(HaveOccurred())
				return
			}
			return
		}
	})

	It("Should NOT get AWS IAM credentials [AWS-IAM] [Zalando]", func() {
		ns := f.Namespace.Name

		By("Creating a awscli POD in namespace " + ns)
		pod := createAWSCLIPod("no-aws-iam-", ns, E2ES3AWSIAMBucket())
		_, err := cs.CoreV1().Pods(ns).Create(pod)
		Expect(err).NotTo(HaveOccurred())
		framework.ExpectNoError(f.WaitForPodRunning(pod.Name))

		// wait for pod to access s3 and POD exit code 0
		for {
			p, err := cs.CoreV1().Pods(ns).Get(pod.Name, metav1.GetOptions{})
			if err != nil {
				Expect(fmt.Errorf("Could not get POD %s", pod.Name)).NotTo(HaveOccurred())
				return
			}

			if p.Status.ContainerStatuses[0].State.Terminated == nil {
				time.Sleep(10 * time.Second)
				continue
			}

			n := p.Status.ContainerStatuses[0].State.Terminated.ExitCode
			if n < 1 {
				// set error
				Expect(fmt.Errorf("expected the s3 bucket access to fail")).NotTo(HaveOccurred())
				return
			}
			return
		}
	})
})
