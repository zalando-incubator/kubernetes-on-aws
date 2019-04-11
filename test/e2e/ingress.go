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
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = framework.KubeDescribe("Ingress ALB creation", func() {
	f := framework.NewDefaultFramework("ingress")
	var (
		cs  kubernetes.Interface
		jig *framework.IngressTestJig
	)
	BeforeEach(func() {
		jig = framework.NewIngressTestJig(f.ClientSet)
		cs = f.ClientSet
	})

	It("Should create valid https and http ALB endpoint [Ingress] [Zalando]", func() {
		serviceName := "ingress-test"
		nameprefix := serviceName + "-"
		ns := f.Namespace.Name
		hostName := fmt.Sprintf("%s-%d.%s", serviceName, time.Now().UTC().Unix(), e2eHostedZone())
		labels := map[string]string{
			"app": serviceName,
		}
		port := 83
		targetPort := 80

		// SVC
		By("Creating service " + serviceName + " in namespace " + ns)
		service := createServiceTypeClusterIP(serviceName, labels, port, targetPort)
		defer func() {
			By("deleting the service")
			defer GinkgoRecover()
			err2 := cs.Core().Services(ns).Delete(service.Name, metav1.NewDeleteOptions(0))
			Expect(err2).NotTo(HaveOccurred())
		}()
		_, err := cs.Core().Services(ns).Create(service)
		Expect(err).NotTo(HaveOccurred())

		// POD
		By("Creating a POD with prefix " + nameprefix + " in namespace " + ns)
		pod := createNginxPod(nameprefix, ns, labels, targetPort)
		defer func() {
			By("deleting the pod")
			defer GinkgoRecover()
			err2 := cs.Core().Pods(ns).Delete(pod.Name, metav1.NewDeleteOptions(0))
			Expect(err2).NotTo(HaveOccurred())
		}()

		_, err = cs.Core().Pods(ns).Create(pod)
		Expect(err).NotTo(HaveOccurred())
		framework.ExpectNoError(f.WaitForPodRunning(pod.Name))

		// Ingress
		By("Creating an ingress with name " + serviceName + " in namespace " + ns + " with hostname " + hostName)
		ing := createIngress(serviceName, hostName, ns, labels, port)
		defer func() {
			By("deleting the ingress")
			defer GinkgoRecover()
			err2 := cs.Extensions().Ingresses(ns).Delete(ing.Name, metav1.NewDeleteOptions(0))
			Expect(err2).NotTo(HaveOccurred())
		}()
		ingressCreate, err := cs.Extensions().Ingresses(ns).Create(ing)
		Expect(err).NotTo(HaveOccurred())
		addr, err := jig.WaitForIngressAddress(cs, ns, ingressCreate.Name, 10*time.Minute)
		Expect(err).NotTo(HaveOccurred())
		ingress, err := cs.Extensions().Ingresses(ns).Get(ing.Name, metav1.GetOptions{ResourceVersion: "0"})
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("ALB endpoint from ingress status: %s", ingress.Status.LoadBalancer.Ingress[0].Hostname))

		//  skipper http -> https redirect
		By("Waiting for skipper route to default redirect from http to https, to see that our ingress-controller and skipper works")
		err = waitForResponse(addr, "http", 10*time.Minute, isRedirect, true)
		Expect(err).NotTo(HaveOccurred())

		// ALB ready
		By("Waiting for ALB to create endpoint " + addr + " and skipper route, to see that our ingress-controller and skipper works")
		err = waitForResponse(addr, "https", 10*time.Minute, isSuccess, true)
		Expect(err).NotTo(HaveOccurred())

		// DNS ready
		By("Waiting for DNS to see that external-dns and skipper route to service and pod works")
		err = waitForResponse(hostName, "https", 10*time.Minute, isSuccess, false)
		Expect(err).NotTo(HaveOccurred())
	})
})

var __ = framework.KubeDescribe("Ingress tests", func() {
	f := framework.NewDefaultFramework("skipper-ingress")
	var (
		cs  kubernetes.Interface
		jig *framework.IngressTestJig
	)

	It("Should create simple ingress [sszuecs] [Ingress] [Zalando]", func() {
		jig = framework.NewIngressTestJig(f.ClientSet)
		cs = f.ClientSet
		serviceName := "skipper-ingress-test"
		//nameprefix := serviceName + "-"
		ns := f.Namespace.Name
		hostName := fmt.Sprintf("%s-%d.%s", serviceName, time.Now().UTC().Unix(), e2eHostedZone())
		labels := map[string]string{
			"app": serviceName,
		}
		port := 8080
		replicas := int32(3)
		targetPort := 9090
		backendContent := "mytest"
		route := fmt.Sprintf(`* -> inlineContent("%s") -> <shunt>`, backendContent)
		waitTime := 10 * time.Minute

		// backend deployment
		By("Creating a deployment with " + serviceName + " in namespace " + ns)
		depl := createSkipperBackendDeployment(serviceName, ns, route, labels, int32(targetPort), replicas)
		deployment, err := cs.Apps().Deployments(ns).Create(depl)
		defer func() {
			By("deleting the deployment")
			defer GinkgoRecover()
			err2 := cs.Apps().Deployments(ns).Delete(deployment.Name, metav1.NewDeleteOptions(0))
			Expect(err2).NotTo(HaveOccurred())
		}()
		Expect(err).NotTo(HaveOccurred())

		By("Creating service " + serviceName + " in namespace " + ns)
		service := createServiceTypeClusterIP(serviceName, labels, port, targetPort)
		_, err = cs.Core().Services(ns).Create(service)
		Expect(err).NotTo(HaveOccurred())

		ing := createIngress(serviceName, hostName, ns, labels, port)
		ingressCreate, err := cs.Extensions().Ingresses(ns).Create(ing)
		Expect(err).NotTo(HaveOccurred())

		addr, err := jig.WaitForIngressAddress(cs, ns, ingressCreate.Name, waitTime)
		Expect(err).NotTo(HaveOccurred())

		_, err = cs.Extensions().Ingresses(ns).Get(ing.Name, metav1.GetOptions{ResourceVersion: "0"})
		Expect(err).NotTo(HaveOccurred())

		//  skipper http -> https redirect
		By("Waiting for skipper route to default redirect from http to https, to see that our ingress-controller and skipper works")
		err = waitForResponse(addr, "http", waitTime, isRedirect, true)
		Expect(err).NotTo(HaveOccurred())

		// ALB ready
		By("Waiting for ALB to create endpoint " + addr + " and skipper route, to see that our ingress-controller and skipper works")
		err = waitForResponse(addr, "https", waitTime, isSuccess, true)
		Expect(err).NotTo(HaveOccurred())

		// DNS ready
		By("Waiting for DNS to see that external-dns and skipper route to service and pod works")
		err = waitForResponse(hostName, "https", waitTime, isSuccess, false)
		Expect(err).NotTo(HaveOccurred())

		// Test that we get content from the default ingress
		By("By checking the content of the reply we see that the ingress stack works")
		rt, quit := createHTTPRoundTripper()
		defer func() {
			quit <- struct{}{}
		}()
		url := "https://" + hostName + "/"
		req, err := http.NewRequest("GET", url, nil)
		Expect(err).NotTo(HaveOccurred())
		resp, err := rt.RoundTrip(req)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			log.Fatalf("Failed to get the right response code from backend: %d", resp.StatusCode)
		}
		b := make([]byte, 0, 1024)
		buf := bytes.NewBuffer(b)
		if n, err := io.Copy(buf, resp.Body); err != nil {
			log.Fatalf("Failed to copy body: %v", err)
		} else {
			log.Printf("copy %d bytes of data: %s", n, buf)
		}
		if s := buf.String(); s != backendContent {
			log.Fatalf("Failed to get the right content got: %s, expected: %s", s, backendContent)
		}
	})
})
