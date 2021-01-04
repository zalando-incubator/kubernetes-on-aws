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
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/ingress"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

var _ = framework.KubeDescribe("Ingress ALB creation", func() {
	f := framework.NewDefaultFramework("ingress")
	var (
		cs  kubernetes.Interface
		jig *ingress.TestJig
	)
	BeforeEach(func() {
		jig = ingress.NewIngressTestJig(f.ClientSet)
		cs = f.ClientSet
	})

	It("Should create valid https and http ALB endpoint [Ingress] [Zalando]", func() {
		serviceName := "ingress-test"
		nameprefix := serviceName + "-"
		ns := f.Namespace.Name
		hostName := fmt.Sprintf("%s-%d.%s", serviceName, time.Now().UTC().Unix(), E2EHostedZone())
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
			err2 := cs.CoreV1().Services(ns).Delete(context.TODO(), service.Name, metav1.DeleteOptions{})
			Expect(err2).NotTo(HaveOccurred())
		}()
		_, err := cs.CoreV1().Services(ns).Create(context.TODO(), service, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		// POD
		By("Creating a POD with prefix " + nameprefix + " in namespace " + ns)
		route := fmt.Sprintf(`* -> inlineContent("%s") -> <shunt>`, "OK")
		pod := createSkipperPod(nameprefix, ns, route, labels, targetPort)
		defer func() {
			By("deleting the pod")
			defer GinkgoRecover()
			err2 := cs.CoreV1().Pods(ns).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
			Expect(err2).NotTo(HaveOccurred())
		}()

		_, err = cs.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		framework.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(f.ClientSet, pod.Name, pod.Namespace))

		// Ingress
		By("Creating an ingress with name " + serviceName + " in namespace " + ns + " with hostname " + hostName)
		ing := createIngress(serviceName, hostName, ns, labels, nil, port)
		defer func() {
			By("deleting the ingress")
			defer GinkgoRecover()
			err2 := cs.NetworkingV1beta1().Ingresses(ns).Delete(context.TODO(), ing.Name, metav1.DeleteOptions{})
			Expect(err2).NotTo(HaveOccurred())
		}()
		ingressCreate, err := cs.NetworkingV1beta1().Ingresses(ns).Create(context.TODO(), ing, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		addr, err := jig.WaitForIngressAddress(cs, ns, ingressCreate.Name, 10*time.Minute)
		Expect(err).NotTo(HaveOccurred())
		ingress, err := cs.NetworkingV1beta1().Ingresses(ns).Get(context.TODO(), ing.Name, metav1.GetOptions{ResourceVersion: "0"})
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
		jig *ingress.TestJig
	)

	It("Should create simple ingress [Ingress] [Zalando]", func() {
		jig = ingress.NewIngressTestJig(f.ClientSet)
		cs = f.ClientSet
		serviceName := "skipper-ingress-test"
		ns := f.Namespace.Name
		hostName := fmt.Sprintf("%s-%d.%s", serviceName, time.Now().UTC().Unix(), E2EHostedZone())
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
		_, err := cs.AppsV1().Deployments(ns).Create(context.TODO(), depl, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Creating service " + serviceName + " in namespace " + ns)
		service := createServiceTypeClusterIP(serviceName, labels, port, targetPort)
		_, err = cs.CoreV1().Services(ns).Create(context.TODO(), service, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		ing := createIngress(serviceName, hostName, ns, labels, nil, port)
		ingressCreate, err := cs.NetworkingV1beta1().Ingresses(ns).Create(context.TODO(), ing, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		addr, err := jig.WaitForIngressAddress(cs, ns, ingressCreate.Name, waitTime)
		Expect(err).NotTo(HaveOccurred())

		_, err = cs.NetworkingV1beta1().Ingresses(ns).Get(context.TODO(), ing.Name, metav1.GetOptions{ResourceVersion: "0"})
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
		Expect(s).To(Equal(backendContent))

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
		ingressUpdate, err := cs.NetworkingV1beta1().Ingresses(ingressCreate.ObjectMeta.Namespace).Update(context.TODO(), updatedIng, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("Waiting for ingress %s/%s we wait to get a 200 with the right content for the next request", ingressUpdate.Namespace, ingressUpdate.Name))
		resp, err = getAndWaitResponse(rt, req, 10*time.Second, http.StatusOK)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		s, err = getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Equal(backendContent))

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
		ingressUpdate, err = cs.NetworkingV1beta1().Ingresses(ingressCreate.ObjectMeta.Namespace).Update(context.TODO(), updatedIng, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("Waiting for ingress %s/%s we wait to get a 404 for the next request", ingressUpdate.Namespace, ingressUpdate.Name))
		resp, err = getAndWaitResponse(rt, req, 10*time.Second, http.StatusNotFound)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

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
		ingressUpdate, err = cs.NetworkingV1beta1().Ingresses(ingressCreate.ObjectMeta.Namespace).Update(context.TODO(), updatedIng, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("Waiting for ingress %s/%s we wait to get a 200 with %s header set to %s for the next request", ingressUpdate.Namespace, ingressUpdate.Name, headerKey, headerVal))
		time.Sleep(10 * time.Second) // wait for routing change propagation
		resp, err = getAndWaitResponse(rt, req, 10*time.Second, http.StatusOK)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(resp.Header.Get(headerKey)).To(Equal(headerVal))
		s, err = getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Equal(backendContent))

		// Test additional hostname
		additionalHostname := fmt.Sprintf("foo-%d.%s", time.Now().UTC().Unix(), E2EHostedZone())
		addHostIng := addHostIngress(updatedIng, additionalHostname)
		ingressUpdate, err = cs.NetworkingV1beta1().Ingresses(ingressCreate.ObjectMeta.Namespace).Update(context.TODO(), addHostIng, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred())
		By("Waiting for new DNS hostname to be resolvable " + additionalHostname)
		err = waitForResponse(additionalHostname, "https", waitTime, isSuccess, false)
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("Testing the old hostname %s for ingress %s/%s we make sure old routes are working", hostName, ingressUpdate.Namespace, ingressUpdate.Name))
		resp, err = getAndWaitResponse(rt, req, 10*time.Second, http.StatusOK)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		s, err = getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Equal(backendContent))
		By(fmt.Sprintf("Testing the new hostname %s for ingress %s/%s we make sure old routes are working", additionalHostname, ingressUpdate.Namespace, ingressUpdate.Name))
		url = "https://" + additionalHostname + "/"
		req, err = http.NewRequest("GET", url, nil)
		Expect(err).NotTo(HaveOccurred())
		resp, err = getAndWaitResponse(rt, req, 10*time.Second, http.StatusOK)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		s, err = getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Equal(backendContent))

		// Test changed path
		newPath := "/foo"
		changePathIng := changePathIngress(updatedIng, newPath)
		ingressUpdate, err = cs.NetworkingV1beta1().Ingresses(ingressCreate.ObjectMeta.Namespace).Update(context.TODO(), changePathIng, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Waiting for ingress %s/%s we wait to get a 404 for the old request, because of the path route", ingressUpdate.Namespace, ingressUpdate.Name))
		resp, err = getAndWaitResponse(rt, req, 10*time.Second, http.StatusNotFound)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		pathURL := "https://" + hostName + newPath
		pathReq, err := http.NewRequest("GET", pathURL, nil)
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("Waiting for ingress %s/%s we wait to get a 200 for a new request to the path route", ingressUpdate.Namespace, ingressUpdate.Name))
		resp, err = getAndWaitResponse(rt, pathReq, 10*time.Second, http.StatusOK)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		s, err = getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Equal(backendContent))
	})
})

var ___ = framework.KubeDescribe("Ingress tests paths", func() {
	f := framework.NewDefaultFramework("skipper-ingress-paths")
	var (
		cs  kubernetes.Interface
		jig *ingress.TestJig
	)

	It("Should create path routes ingress [Ingress] [Zalando]", func() {
		jig = ingress.NewIngressTestJig(f.ClientSet)
		cs = f.ClientSet
		serviceName := "skipper-ingress-test-pr"
		serviceName2 := "skipper-ingress-test-pr2"
		ns := f.Namespace.Name
		hostName := fmt.Sprintf("%s-%d.%s", serviceName, time.Now().UTC().Unix(), E2EHostedZone())
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
		_, err := cs.AppsV1().Deployments(ns).Create(context.TODO(), depl, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		By("Creating a 2nd deployment with " + serviceName2 + " in namespace " + ns)
		depl2 := createSkipperBackendDeployment(serviceName2, ns, route2, labels2, int32(targetPort), replicas)
		_, err = cs.AppsV1().Deployments(ns).Create(context.TODO(), depl2, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Creating service " + serviceName + " in namespace " + ns)
		service := createServiceTypeClusterIP(serviceName, labels, port, targetPort)
		_, err = cs.CoreV1().Services(ns).Create(context.TODO(), service, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Creating service " + serviceName2 + " in namespace " + ns)
		service2 := createServiceTypeClusterIP(serviceName2, labels2, port, targetPort)
		_, err = cs.CoreV1().Services(ns).Create(context.TODO(), service2, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Creating ingress " + serviceName + " in namespace " + ns + "with hostname " + hostName)
		ing := createIngress(serviceName, hostName, ns, labels, nil, port)
		ingressCreate, err := cs.NetworkingV1beta1().Ingresses(ns).Create(context.TODO(), ing, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		addr, err := jig.WaitForIngressAddress(cs, ns, ingressCreate.Name, waitTime)
		Expect(err).NotTo(HaveOccurred())

		_, err = cs.NetworkingV1beta1().Ingresses(ns).Get(context.TODO(), ing.Name, metav1.GetOptions{ResourceVersion: "0"})
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
		Expect(s).To(Equal(backendContent))

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
		ingressUpdate, err := cs.NetworkingV1beta1().Ingresses(ingressCreate.ObjectMeta.Namespace).Update(context.TODO(), updatedIng, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred())
		// wait 20 seconds to ensure the ingress change is applied by
		// all skippers
		time.Sleep(20 * time.Second)

		By(fmt.Sprintf("Testing for ingress %s/%s we want to get a 404 for path /", ingressUpdate.Namespace, ingressUpdate.Name))
		resp, err = getAndWaitResponse(rt, req, 10*time.Second, http.StatusNotFound)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

		By(fmt.Sprintf("Testing for ingress %s/%s we want to get a 200 for path %s", ingressUpdate.Namespace, ingressUpdate.Name, bepath))
		beurl := "https://" + hostName + bepath
		bereq, err := http.NewRequest("GET", beurl, nil)
		Expect(err).NotTo(HaveOccurred())
		resp, err = getAndWaitResponse(rt, bereq, 10*time.Second, http.StatusOK)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		s, err = getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Equal(backendContent))

		// Test ingress with 2 paths
		bepath2 := "/bar"
		beurl2 := "https://" + hostName + bepath2
		bereq2, err := http.NewRequest("GET", beurl2, nil)
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("Testing for ingress %s/%s we want to get a 404 for path %s", ingressUpdate.Namespace, ingressUpdate.Name, bepath2))
		resp, err = getAndWaitResponse(rt, bereq2, 10*time.Second, http.StatusNotFound)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		By(fmt.Sprintf("Testing for ingress %s/%s we want to get a 200 for path %s", ingressUpdate.Namespace, ingressUpdate.Name, bepath2))
		updatedIng = addPathIngress(updatedIng,
			bepath2,
			v1beta1.IngressBackend{
				ServiceName: serviceName2,
				ServicePort: intstr.FromInt(port),
			},
		)
		ingressUpdate, err = cs.NetworkingV1beta1().Ingresses(ingressCreate.ObjectMeta.Namespace).Update(context.TODO(), updatedIng, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred())
		// wait 20 seconds to ensure the ingress change is applied by
		// all skippers
		time.Sleep(20 * time.Second)
		resp, err = getAndWaitResponse(rt, bereq2, 10*time.Second, http.StatusOK)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		s, err = getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Equal(backendContent2))

		By(fmt.Sprintf("Testing for ingress %s/%s we want to get a 200 for path %s without change from the other path", ingressUpdate.Namespace, ingressUpdate.Name, bepath))
		beurl = "https://" + hostName + bepath
		bereq, err = http.NewRequest("GET", beurl, nil)
		resp, err = getAndWaitResponse(rt, bereq, 10*time.Second, http.StatusOK)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		s, err = getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Equal(backendContent))
	})
})

var ____ = framework.KubeDescribe("Ingress tests custom routes", func() {
	f := framework.NewDefaultFramework("skipper-ingress-custom")
	var (
		cs  kubernetes.Interface
		jig *ingress.TestJig
	)

	It("Should create custom routes ingress [Ingress] [Zalando]", func() {
		jig = ingress.NewIngressTestJig(f.ClientSet)
		cs = f.ClientSet
		serviceName := "skipper-ingress-test-custom"
		ns := f.Namespace.Name
		hostName := fmt.Sprintf("%s-%d.%s", serviceName, time.Now().UTC().Unix(), E2EHostedZone())
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
		_, err := cs.AppsV1().Deployments(ns).Create(context.TODO(), depl, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Creating service " + serviceName + " in namespace " + ns)
		service := createServiceTypeClusterIP(serviceName, labels, port, targetPort)
		_, err = cs.CoreV1().Services(ns).Create(context.TODO(), service, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Creating ingress " + serviceName + " in namespace " + ns + "with hostname " + hostName)
		ing := createIngress(serviceName, hostName, ns, labels, nil, port)
		ingressCreate, err := cs.NetworkingV1beta1().Ingresses(ns).Create(context.TODO(), ing, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		addr, err := jig.WaitForIngressAddress(cs, ns, ingressCreate.Name, waitTime)
		Expect(err).NotTo(HaveOccurred())

		_, err = cs.NetworkingV1beta1().Ingresses(ns).Get(context.TODO(), ing.Name, metav1.GetOptions{ResourceVersion: "0"})
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
		Expect(s).To(Equal(backendContent))

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
		ingressUpdate, err := cs.NetworkingV1beta1().Ingresses(ingressCreate.ObjectMeta.Namespace).Update(context.TODO(), updatedIng, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred())
		// wait 20 seconds to ensure the ingress change is applied by
		// all skippers
		time.Sleep(20 * time.Second)

		By(fmt.Sprintf("Testing for ingress %s/%s we want to get a 307 for path %s", ingressUpdate.Namespace, ingressUpdate.Name, redirectPath))
		req, err = http.NewRequest("GET", redirectURL, nil)
		Expect(err).NotTo(HaveOccurred())
		resp, err = getAndWaitResponse(rt, req, 10*time.Second, http.StatusTemporaryRedirect)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusTemporaryRedirect))

		reqRedirectURL := resp.Header.Get("Location")
		By(fmt.Sprintf("Testing for ingress %s/%s rediretc Location we want to get a 200 for URL %s", ingressUpdate.Namespace, ingressUpdate.Name, reqRedirectURL))
		Expect(redirectDestinationURL).To(Equal(reqRedirectURL))
		redirectreq, err := http.NewRequest("GET", reqRedirectURL, nil)
		resp, err = getAndWaitResponse(rt, redirectreq, 10*time.Second, http.StatusOK)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		s, err = getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Equal(backendContent))
	})
})

var _____ = framework.KubeDescribe("Ingress tests simple NLB", func() {
	f := framework.NewDefaultFramework("skipper-ingress-simple-nlb")
	var (
		cs  kubernetes.Interface
		jig *ingress.TestJig
	)

	It("Should create simple NLB ingress [Ingress] [Zalando]", func() {
		jig = ingress.NewIngressTestJig(f.ClientSet)
		cs = f.ClientSet
		serviceName := "skipper-ingress-test"
		ns := f.Namespace.Name
		hostName := fmt.Sprintf("%s-%d.%s", serviceName, time.Now().UTC().Unix(), E2EHostedZone())
		labels := map[string]string{
			"app": serviceName,
		}
		annotations := map[string]string{
			"zalando.org/aws-load-balancer-type": "nlb",
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
		_, err := cs.AppsV1().Deployments(ns).Create(context.TODO(), depl, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Creating service " + serviceName + " in namespace " + ns)
		service := createServiceTypeClusterIP(serviceName, labels, port, targetPort)
		_, err = cs.CoreV1().Services(ns).Create(context.TODO(), service, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		ing := createIngress(serviceName, hostName, ns, labels, annotations, port)
		ingressCreate, err := cs.NetworkingV1beta1().Ingresses(ns).Create(context.TODO(), ing, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		addr, err := jig.WaitForIngressAddress(cs, ns, ingressCreate.Name, waitTime)
		Expect(err).NotTo(HaveOccurred())

		_, err = cs.NetworkingV1beta1().Ingresses(ns).Get(context.TODO(), ing.Name, metav1.GetOptions{ResourceVersion: "0"})
		Expect(err).NotTo(HaveOccurred())

		// //  skipper http -> https redirect
		// By("Waiting for skipper route to default redirect from http to https, to see that our ingress-controller and skipper works")
		// err = waitForResponse(addr, "http", waitTime, isRedirect, true)
		// Expect(err).NotTo(HaveOccurred())

		// NLB ready
		By("Waiting for NLB to create endpoint " + addr + " and skipper route, to see that our ingress-controller and skipper works")
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
		Expect(s).To(Equal(backendContent))
	})
})
