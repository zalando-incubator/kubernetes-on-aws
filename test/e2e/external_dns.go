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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	externalDNSAnnotation = "external-dns.alpha.kubernetes.io/hostname"
)

var _ = framework.KubeDescribe("External DNS creation", func() {
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
			err := cs.Core().Services(ns).Delete(serviceName, nil)
			Expect(err).NotTo(HaveOccurred())
		}()

		hostName := fmt.Sprintf("%s-%d.%s", serviceName, time.Now().UTC().Unix(), e2eHostedZone())
		service := createServiceTypeLoadbalancer(serviceName, hostName, labels, port)

		_, err := cs.Core().Services(ns).Create(service)
		Expect(err).NotTo(HaveOccurred())

		By("Submitting the pod to kubernetes")
		pod := createNginxPod(nameprefix, ns, labels, port)
		defer func() {
			By("deleting the pod")
			defer GinkgoRecover()
			err2 := cs.Core().Pods(ns).Delete(pod.Name, metav1.NewDeleteOptions(0))
			Expect(err2).NotTo(HaveOccurred())
		}()

		_, err = cs.Core().Pods(ns).Create(pod)
		Expect(err).NotTo(HaveOccurred())

		framework.ExpectNoError(f.WaitForPodRunning(pod.Name))

		timeout := 10 * time.Minute
		// wait for DNS and for pod to be reachable.
		By("Waiting up to " + timeout.String() + " for " + hostName + " to be reachable")
		err = waitForSuccessfulResponse(hostName, timeout)
		Expect(err).NotTo(HaveOccurred())
	})
})
