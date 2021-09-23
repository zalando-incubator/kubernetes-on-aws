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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	externalDNSAnnotation = "external-dns.alpha.kubernetes.io/hostname"
)

var _ = describe("External DNS creation", func() {
	f := framework.NewDefaultFramework("external-dns")
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("Should create DNS entry [Slow] [Zalando]", func() {
		// TODO: use the ServiceTestJig here
		serviceName := "external-dns-test"
		nameprefix := serviceName + "-"
		ns := f.Namespace.Name
		labels := map[string]string{
			"foo": "bar",
			"baz": "blah",
		}
		port := 80

		By("Creating service " + serviceName + " in namespace " + ns)
		defer func() {
			err := cs.CoreV1().Services(ns).Delete(context.TODO(), serviceName, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}()

		hostName := fmt.Sprintf("%s-%d.%s", serviceName, time.Now().UTC().Unix(), E2EHostedZone())
		service := createServiceTypeLoadbalancer(serviceName, hostName, labels, port)

		_, err := cs.CoreV1().Services(ns).Create(context.TODO(), service, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Submitting the pod to kubernetes")
		route := fmt.Sprintf(`* -> inlineContent("%s") -> <shunt>`, "OK")
		pod := createSkipperPod(nameprefix, ns, route, labels, port)
		defer func() {
			By("deleting the pod")
			defer GinkgoRecover()
			err2 := cs.CoreV1().Pods(ns).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
			Expect(err2).NotTo(HaveOccurred())
		}()

		_, err = cs.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		framework.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(f.ClientSet, pod.Name, pod.Namespace))

		timeout := 10 * time.Minute
		// wait for DNS and for pod to be reachable.
		By("Waiting up to " + timeout.String() + " for " + hostName + " to be reachable")
		err = waitForSuccessfulResponse(hostName, timeout)
		Expect(err).NotTo(HaveOccurred())
	})
})
