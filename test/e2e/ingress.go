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
	"log"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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
		err = waitForResponse(addr, "https", 10*time.Minute, isNotFound, true)
		Expect(err).NotTo(HaveOccurred())

		// DNS ready
		By("Waiting for DNS to see that external-dns and skipper route to service and pod works")
		err = waitForResponse(hostName, "https", 10*time.Minute, isSuccess, false)
		Expect(err).NotTo(HaveOccurred())
	})
})

var __ = framework.KubeDescribe("Ingress tests simple", func() {
	f := framework.NewDefaultFramework("skipper-ingress-simple")
	var (
		cs  kubernetes.Interface
		jig *framework.IngressTestJig
	)

	It("Should create simple ingress [Ingress] [Zalando]", func() {
		jig = framework.NewIngressTestJig(f.ClientSet)
		cs = f.ClientSet
		serviceName := "skipper-ingress-test"
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

		// CREATE setup
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
		err = waitForResponse(addr, "https", waitTime, isNotFound, true)
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
		s, err := getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		if s != backendContent {
			log.Fatalf("Failed to get the right content got: %s, expected: %s", s, backendContent)
		}

		// Start actual ingress tests
		// Test ingress Predicates with Method("GET")
		path := "/"
		updatedIng := updateIngress(ingressCreate.ObjectMeta.Name,
			ingressCreate.ObjectMeta.Namespace,
			hostName,
			serviceName,
			path,
			ingressCreate.ObjectMeta.Labels,
			map[string]string{
				"zalando.org/skipper-predicate": `Method("GET")`,
			},
			port,
		)
		ingressUpdate, err := cs.Extensions().Ingresses(ingressCreate.ObjectMeta.Namespace).Update(updatedIng)
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("Waiting for ingress %s/%s we wait to get a 200 with the right content for the next request", ingressUpdate.Namespace, ingressUpdate.Name))
		resp, err = getAndWaitResponse(rt, req, 10*time.Second, http.StatusOK)
		Expect(err).NotTo(HaveOccurred())
		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Failed to get status code expected status code 200: %d", resp.StatusCode)
		}
		s, err = getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		if s != backendContent {
			log.Fatalf("Failed to get the right content after update got: %s, expected: %s", s, backendContent)
		}

		// Test ingress Predicates with Method("PUT")
		path = "/"
		updatedIng = updateIngress(ingressCreate.ObjectMeta.Name,
			ingressCreate.ObjectMeta.Namespace,
			hostName,
			serviceName,
			path,
			ingressCreate.ObjectMeta.Labels,
			map[string]string{
				"zalando.org/skipper-predicate": `Method("PUT")`,
			},
			port,
		)
		ingressUpdate, err = cs.Extensions().Ingresses(ingressCreate.ObjectMeta.Namespace).Update(updatedIng)
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("Waiting for ingress %s/%s we wait to get a 404 for the next request", ingressUpdate.Namespace, ingressUpdate.Name))
		resp, err = getAndWaitResponse(rt, req, 10*time.Second, http.StatusNotFound)
		Expect(err).NotTo(HaveOccurred())
		if resp.StatusCode != http.StatusNotFound {
			log.Fatalf("Failed to get the right the right status code 404, got: %d", resp.StatusCode)
		}

		// Test ingress Filters
		path = "/"
		headerKey := "X-Foo"
		headerVal := "f00"
		updatedIng = updateIngress(ingressCreate.ObjectMeta.Name,
			ingressCreate.ObjectMeta.Namespace,
			hostName,
			serviceName,
			path,
			ingressCreate.ObjectMeta.Labels,
			map[string]string{
				"zalando.org/skipper-filter": fmt.Sprintf(`setResponseHeader("%s", "%s")`, headerKey, headerVal),
			},
			port,
		)
		ingressUpdate, err = cs.Extensions().Ingresses(ingressCreate.ObjectMeta.Namespace).Update(updatedIng)
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("Waiting for ingress %s/%s we wait to get a 200 with %s header set to %s for the next request", ingressUpdate.Namespace, ingressUpdate.Name, headerKey, headerVal))
		time.Sleep(10 * time.Second) // wait for routing change propagation
		resp, err = getAndWaitResponse(rt, req, 10*time.Second, http.StatusOK)
		Expect(err).NotTo(HaveOccurred())
		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Failed to get the right the right status code 200, got: %d", resp.StatusCode)
		}
		if got := resp.Header.Get(headerKey); got != headerVal {
			log.Fatalf("Failed to get Header, got: %s, want: %s", got, headerVal)
		}
		s, err = getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		if s != backendContent {
			log.Fatalf("Failed to get the right content after update got: %s, expected: %s", s, backendContent)
		}

		// Test additional hostname
		additionalHostname := fmt.Sprintf("foo-%d.%s", time.Now().UTC().Unix(), e2eHostedZone())
		addHostIng := addHostIngress(updatedIng, additionalHostname)
		ingressUpdate, err = cs.Extensions().Ingresses(ingressCreate.ObjectMeta.Namespace).Update(addHostIng)
		Expect(err).NotTo(HaveOccurred())
		By("Waiting for new DNS hostname to be resolvable " + additionalHostname)
		err = waitForResponse(additionalHostname, "https", waitTime, isSuccess, false)
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("Testing the old hostname %s for ingress %s/%s we make sure old routes are working", hostName, ingressUpdate.Namespace, ingressUpdate.Name))
		resp, err = getAndWaitResponse(rt, req, 10*time.Second, http.StatusOK)
		Expect(err).NotTo(HaveOccurred())
		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Failed to get the right the right status code 200, got: %d", resp.StatusCode)
		}
		s, err = getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		if s != backendContent {
			log.Fatalf("Failed to get the right content after update got: %s, expected: %s", s, backendContent)
		}
		By(fmt.Sprintf("Testing the new hostname %s for ingress %s/%s we make sure old routes are working", additionalHostname, ingressUpdate.Namespace, ingressUpdate.Name))
		url = "https://" + additionalHostname + "/"
		req, err = http.NewRequest("GET", url, nil)
		Expect(err).NotTo(HaveOccurred())
		resp, err = getAndWaitResponse(rt, req, 10*time.Second, http.StatusOK)
		Expect(err).NotTo(HaveOccurred())
		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Failed to get the right the right status code 200, got: %d", resp.StatusCode)
		}
		s, err = getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		if s != backendContent {
			log.Fatalf("Failed to get the right content after update got: %s, expected: %s", s, backendContent)
		}

		// Test changed path
		newPath := "/foo"
		changePathIng := changePathIngress(updatedIng, newPath)
		ingressUpdate, err = cs.Extensions().Ingresses(ingressCreate.ObjectMeta.Namespace).Update(changePathIng)
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Waiting for ingress %s/%s we wait to get a 404 for the old request, because of the path route", ingressUpdate.Namespace, ingressUpdate.Name))
		resp, err = getAndWaitResponse(rt, req, 10*time.Second, http.StatusNotFound)
		Expect(err).NotTo(HaveOccurred())
		if resp.StatusCode != http.StatusNotFound {
			log.Fatalf("Failed to get the right the right status code 404, got: %d", resp.StatusCode)
		}
		pathURL := "https://" + hostName + newPath
		pathReq, err := http.NewRequest("GET", pathURL, nil)
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("Waiting for ingress %s/%s we wait to get a 200 for a new request to the path route", ingressUpdate.Namespace, ingressUpdate.Name))
		resp, err = getAndWaitResponse(rt, pathReq, 10*time.Second, http.StatusOK)
		Expect(err).NotTo(HaveOccurred())
		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Failed to get the right the right for %s status code 200, got: %d", newPath, resp.StatusCode)
		}
		s, err = getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		if s != backendContent {
			log.Fatalf("Failed to get the right content from %s after update got: %s, expected: %s", newPath, s, backendContent)
		}

	})
})

var ___ = framework.KubeDescribe("Ingress tests paths", func() {
	f := framework.NewDefaultFramework("skipper-ingress-paths")
	var (
		cs  kubernetes.Interface
		jig *framework.IngressTestJig
	)

	It("Should create path routes ingress [Ingress] [Zalando]", func() {
		jig = framework.NewIngressTestJig(f.ClientSet)
		cs = f.ClientSet
		serviceName := "skipper-ingress-test-pr"
		serviceName2 := "skipper-ingress-test-pr2"
		ns := f.Namespace.Name
		hostName := fmt.Sprintf("%s-%d.%s", serviceName, time.Now().UTC().Unix(), e2eHostedZone())
		labels := map[string]string{
			"app": serviceName,
		}
		labels2 := map[string]string{
			"app": serviceName2,
		}
		port := 8080
		replicas := int32(3)
		targetPort := 9090
		backendContent := "be-foo"
		backendContent2 := "be-bar"
		route := fmt.Sprintf(`* -> inlineContent("%s") -> <shunt>`, backendContent)
		route2 := fmt.Sprintf(`* -> inlineContent("%s") -> <shunt>`, backendContent2)
		waitTime := 10 * time.Minute

		// CREATE setup
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
		By("Creating a 2nd deployment with " + serviceName2 + " in namespace " + ns)
		depl2 := createSkipperBackendDeployment(serviceName2, ns, route2, labels2, int32(targetPort), replicas)
		deployment2, err := cs.Apps().Deployments(ns).Create(depl2)
		defer func() {
			By("deleting the deployment")
			defer GinkgoRecover()
			err2 := cs.Apps().Deployments(ns).Delete(deployment2.Name, metav1.NewDeleteOptions(0))
			Expect(err2).NotTo(HaveOccurred())
		}()
		Expect(err).NotTo(HaveOccurred())

		By("Creating service " + serviceName + " in namespace " + ns)
		service := createServiceTypeClusterIP(serviceName, labels, port, targetPort)
		_, err = cs.Core().Services(ns).Create(service)
		Expect(err).NotTo(HaveOccurred())

		By("Creating service " + serviceName2 + " in namespace " + ns)
		service2 := createServiceTypeClusterIP(serviceName2, labels2, port, targetPort)
		_, err = cs.Core().Services(ns).Create(service2)
		Expect(err).NotTo(HaveOccurred())

		By("Creating ingress " + serviceName + " in namespace " + ns + "with hostname " + hostName)
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
		err = waitForResponse(addr, "https", waitTime, isNotFound, true)
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
		s, err := getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		if s != backendContent {
			log.Fatalf("Failed to get the right content got: %s, expected: %s", s, backendContent)
		}

		// Start actual ingress tests
		// Test ingress with 1 path
		bepath := "/foo"
		updatedIng := updateIngress(ingressCreate.ObjectMeta.Name,
			ingressCreate.ObjectMeta.Namespace,
			hostName,
			serviceName,
			bepath,
			ingressCreate.ObjectMeta.Labels,
			ingressCreate.ObjectMeta.Annotations,
			port,
		)
		ingressUpdate, err := cs.Extensions().Ingresses(ingressCreate.ObjectMeta.Namespace).Update(updatedIng)
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Testing for ingress %s/%s we want to get a 404 for path /", ingressUpdate.Namespace, ingressUpdate.Name))
		resp, err = getAndWaitResponse(rt, req, 10*time.Second, http.StatusNotFound)
		Expect(err).NotTo(HaveOccurred())
		if resp.StatusCode != http.StatusNotFound {
			log.Fatalf("Failed to get status code expected status code 404: %d", resp.StatusCode)
		}

		By(fmt.Sprintf("Testing for ingress %s/%s we want to get a 200 for path %s", ingressUpdate.Namespace, ingressUpdate.Name, bepath))
		beurl := "https://" + hostName + bepath
		bereq, err := http.NewRequest("GET", beurl, nil)
		resp, err = getAndWaitResponse(rt, bereq, 10*time.Second, http.StatusOK)
		Expect(err).NotTo(HaveOccurred())
		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Failed to get status code expected status code 200: %d", resp.StatusCode)
		}
		s, err = getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		if s != backendContent {
			log.Fatalf("Failed to get the right content after update got: %s, expected: %s", s, backendContent)
		}

		// Test ingress with 2 paths
		bepath2 := "/bar"
		beurl2 := "https://" + hostName + bepath2
		bereq2, err := http.NewRequest("GET", beurl2, nil)
		By(fmt.Sprintf("Testing for ingress %s/%s we want to get a 404 for path %s", ingressUpdate.Namespace, ingressUpdate.Name, bepath2))
		resp, err = getAndWaitResponse(rt, bereq2, 10*time.Second, http.StatusNotFound)
		Expect(err).NotTo(HaveOccurred())
		if resp.StatusCode != http.StatusNotFound {
			log.Fatalf("Failed to get status code expected status code 404: %d", resp.StatusCode)
		}
		By(fmt.Sprintf("Testing for ingress %s/%s we want to get a 200 for path %s", ingressUpdate.Namespace, ingressUpdate.Name, bepath2))
		updatedIng = addPathIngress(updatedIng,
			bepath2,
			v1beta1.IngressBackend{
				ServiceName: serviceName2,
				ServicePort: intstr.FromInt(port),
			},
		)
		ingressUpdate, err = cs.Extensions().Ingresses(ingressCreate.ObjectMeta.Namespace).Update(updatedIng)
		Expect(err).NotTo(HaveOccurred())
		resp, err = getAndWaitResponse(rt, bereq2, 10*time.Second, http.StatusOK)
		Expect(err).NotTo(HaveOccurred())
		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Failed to get status code expected status code 200: %d", resp.StatusCode)
		}
		s, err = getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		if s != backendContent2 {
			log.Fatalf("Failed to get the right content after update got: %s, expected: %s", s, backendContent2)
		}

		By(fmt.Sprintf("Testing for ingress %s/%s we want to get a 200 for path %s without change from the other path", ingressUpdate.Namespace, ingressUpdate.Name, bepath))
		beurl = "https://" + hostName + bepath
		bereq, err = http.NewRequest("GET", beurl, nil)
		resp, err = getAndWaitResponse(rt, bereq, 10*time.Second, http.StatusOK)
		Expect(err).NotTo(HaveOccurred())
		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Failed to get status code expected status code 200: %d", resp.StatusCode)
		}
		s, err = getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		if s != backendContent {
			log.Fatalf("Failed to get the right content after update got: %s, expected: %s", s, backendContent)
		}
	})
})

var ____ = framework.KubeDescribe("Ingress tests custom routes", func() {
	f := framework.NewDefaultFramework("skipper-ingress-custom")
	var (
		cs  kubernetes.Interface
		jig *framework.IngressTestJig
	)

	It("Should create custom routes ingress [Ingress] [Zalando]", func() {
		jig = framework.NewIngressTestJig(f.ClientSet)
		cs = f.ClientSet
		serviceName := "skipper-ingress-test-custom"
		ns := f.Namespace.Name
		hostName := fmt.Sprintf("%s-%d.%s", serviceName, time.Now().UTC().Unix(), e2eHostedZone())
		labels := map[string]string{
			"app": serviceName,
		}
		port := 8080
		replicas := int32(3)
		targetPort := 9090
		backendContent := "custom-foo"
		route := fmt.Sprintf(`* -> inlineContent("%s") -> <shunt>`, backendContent)
		waitTime := 10 * time.Minute

		// CREATE setup
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

		By("Creating ingress " + serviceName + " in namespace " + ns + "with hostname " + hostName)
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
		err = waitForResponse(addr, "https", waitTime, isNotFound, true)
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
		s, err := getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		if s != backendContent {
			log.Fatalf("Failed to get the right content got: %s, expected: %s", s, backendContent)
		}

		// Start actual ingress tests
		// Test ingress with 1 custom route
		path := "/"
		baseURL := "https://" + hostName
		redirectDestinationURL := baseURL + path
		redirectPath := "/redirect"
		redirectURL := baseURL + redirectPath
		redirectRoute := fmt.Sprintf(`redirecttoself: PathRegexp("%s") -> modPath("%s", "%s") -> redirectTo(307, "%s") -> <shunt>;`, redirectPath, redirectPath, path, redirectDestinationURL)
		updatedIng := updateIngress(ingressCreate.ObjectMeta.Name,
			ingressCreate.ObjectMeta.Namespace,
			hostName,
			serviceName,
			path,
			ingressCreate.ObjectMeta.Labels,
			map[string]string{
				"zalando.org/skipper-routes": redirectRoute,
			},
			port,
		)
		ingressUpdate, err := cs.Extensions().Ingresses(ingressCreate.ObjectMeta.Namespace).Update(updatedIng)
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Testing for ingress %s/%s we want to get a 307 for path %s", ingressUpdate.Namespace, ingressUpdate.Name, redirectPath))
		req, err = http.NewRequest("GET", redirectURL, nil)
		Expect(err).NotTo(HaveOccurred())
		resp, err = getAndWaitResponse(rt, req, 10*time.Second, http.StatusTemporaryRedirect)
		Expect(err).NotTo(HaveOccurred())
		if resp.StatusCode != http.StatusTemporaryRedirect {
			log.Fatalf("Failed to get status code expected status code 307: %d", resp.StatusCode)
		}

		reqRedirectURL := resp.Header.Get("Location")
		By(fmt.Sprintf("Testing for ingress %s/%s rediretc Location we want to get a 200 for URL %s", ingressUpdate.Namespace, ingressUpdate.Name, reqRedirectURL))
		if redirectDestinationURL != reqRedirectURL {
			log.Fatalf("Failed to get the right redirect from header: %s, expected: %s", reqRedirectURL, redirectDestinationURL)
		}
		redirectreq, err := http.NewRequest("GET", reqRedirectURL, nil)
		resp, err = getAndWaitResponse(rt, redirectreq, 10*time.Second, http.StatusOK)
		Expect(err).NotTo(HaveOccurred())
		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Failed to get status code expected status code 200: %d", resp.StatusCode)
		}
		s, err = getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		if s != backendContent {
			log.Fatalf("Failed to get the right content after update got: %s, expected: %s", s, backendContent)
		}
	})
})
