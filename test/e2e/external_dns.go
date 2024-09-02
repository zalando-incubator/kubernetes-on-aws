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
	admissionapi "k8s.io/pod-security-admission/api"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	externalDNSAnnotation = "external-dns.alpha.kubernetes.io/hostname"
)

var _ = describe("External DNS creation", func() {
	f := framework.NewDefaultFramework("external-dns")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelBaseline
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	f.It("Should create DNS entry [Zalando]", f.WithSlow(), func(ctx context.Context) {
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
			err := cs.CoreV1().Services(ns).Delete(ctx, serviceName, metav1.DeleteOptions{})
			framework.ExpectNoError(err)
		}()

		hostName := fmt.Sprintf("%s-%d.%s", serviceName, time.Now().UTC().Unix(), E2EHostedZone())
		service := createServiceTypeLoadbalancer(serviceName, hostName, labels, port)

		_, err := cs.CoreV1().Services(ns).Create(ctx, service, metav1.CreateOptions{})
		framework.ExpectNoError(err)

		By("Submitting the pod to kubernetes")
		route := fmt.Sprintf(`* -> inlineContent("%s") -> <shunt>`, "OK")
		pod := createSkipperPod(nameprefix, ns, route, labels, port)
		defer func() {
			By("deleting the pod")
			defer GinkgoRecover()
			err2 := cs.CoreV1().Pods(ns).Delete(ctx, pod.Name, metav1.DeleteOptions{})
			Expect(err2).NotTo(HaveOccurred())
		}()

		_, err = cs.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{})
		framework.ExpectNoError(err)

		framework.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(ctx, f.ClientSet, pod.Name, pod.Namespace))

		timeout := 10 * time.Minute
		// wait for DNS and for pod to be reachable.
		By("Waiting up to " + timeout.String() + " for " + hostName + " to be reachable")
		err = waitForSuccessfulResponse(hostName, timeout)
		framework.ExpectNoError(err)
	})
})
