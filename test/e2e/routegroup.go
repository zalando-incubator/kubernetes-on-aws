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
	rgclient "github.com/szuecs/routegroup-client"
	rgv1 "github.com/szuecs/routegroup-client/apis/zalando.org/v1"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = framework.KubeDescribe("RouteGroup ALB creation", func() {
	f := framework.NewDefaultFramework("routegroup")
	var (
		cs rgclient.Interface
	)
	BeforeEach(func() {
		By("Creating an rgclient Clientset")
		config, err := framework.LoadConfig()
		Expect(err).NotTo(HaveOccurred())
		config.QPS = f.Options.ClientQPS
		config.Burst = f.Options.ClientBurst
		if f.Options.GroupVersion != nil {
			config.GroupVersion = f.Options.GroupVersion
		}
		cs, err = rgclient.NewClientset(config)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should create valid https and http ALB endpoint [RouteGroup] [Zalando]", func() {
		var resp *http.Response
		serviceName := "rg-test"
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
		_, err := cs.CoreV1().Services(ns).Create(context.TODO(), service, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		// POD
		By("Creating a POD with prefix " + nameprefix + " in namespace " + ns)
		expectedResponse := "OK RG1"
		pod := createSkipperPod(nameprefix, ns, fmt.Sprintf(`r0: * -> inlineContent("%s") -> <shunt>`, expectedResponse), labels, targetPort)

		_, err = cs.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		framework.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(f.ClientSet, pod.Name, pod.Namespace))

		// RouteGroup
		By("Creating a routegroup with name " + serviceName + " in namespace " + ns + " with hostname " + hostName)
		rg := createRouteGroup(serviceName, hostName, ns, labels, nil, port, rgv1.RouteGroupRouteSpec{
			PathSubtree: "/",
		})
		rgCreate, err := cs.ZalandoV1().RouteGroups(ns).Create(context.TODO(), rg, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		addr, err := waitForRouteGroup(cs, rgCreate.Name, rgCreate.Namespace, 10*time.Minute)
		Expect(err).NotTo(HaveOccurred())
		rgGot, err := cs.ZalandoV1().RouteGroups(ns).Get(context.TODO(), rg.Name, metav1.GetOptions{ResourceVersion: "0"})
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("ALB endpoint from routegroup status: %s", rgGot.Status.LoadBalancer.RouteGroup[0].Hostname))

		//  skipper http -> https redirect
		By("Waiting for skipper route to default redirect from http to https, to see that our routegroup-controller and skipper works")
		err = waitForResponse(addr, "http", 10*time.Minute, isRedirect, true)
		Expect(err).NotTo(HaveOccurred())

		// ALB ready
		By("Waiting for ALB to create endpoint " + addr + " and skipper route, to see that our routegroup-controller and skipper works")
		err = waitForResponse(addr, "https", 10*time.Minute, isNotFound, true)
		Expect(err).NotTo(HaveOccurred())

		// DNS ready
		By("Waiting for DNS to see that external-dns and skipper route to service and pod works")
		err = waitForResponse(hostName, "https", 10*time.Minute, isSuccess, false)
		Expect(err).NotTo(HaveOccurred())

		// response is from our backend
		By("checking the response body we know, if we got the response from our backend")
		req, err := http.NewRequest("GET", "https://"+hostName+"/", nil)
		Expect(err).NotTo(HaveOccurred())
		resp, err = waitForResponseReturnResponse(req, 10*time.Minute, isSuccess, false)
		Expect(err).NotTo(HaveOccurred())
		s, err := getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Equal(expectedResponse))
	})

	It("Should create a route with predicates [RouteGroup] [Zalando]", func() {
		var resp *http.Response
		serviceName := "rg-test-pred"
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
		_, err := cs.CoreV1().Services(ns).Create(context.TODO(), service, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		// POD
		By("Creating a POD with prefix " + nameprefix + " in namespace " + ns)
		expectedResponse := "OK RG predicate"
		pod := createSkipperPod(
			nameprefix,
			ns,
			fmt.Sprintf(`rHealth: Path("/") -> inlineContent("OK") -> <shunt>;
rBackend: Path("/backend") -> inlineContent("%s") -> <shunt>;`,
				expectedResponse),
			labels,
			targetPort)

		_, err = cs.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		framework.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(f.ClientSet, pod.Name, pod.Namespace))

		// RouteGroup
		By("Creating a routegroup with name " + serviceName + " in namespace " + ns + " with hostname " + hostName)
		rg := createRouteGroup(serviceName, hostName, ns, labels, nil, port, rgv1.RouteGroupRouteSpec{
			PathSubtree: "/backend",
			Methods:     []rgv1.HTTPMethod{rgv1.MethodGet},
			Predicates:  []string{`Header("Foo", "bar")`},
		})
		rgCreate, err := cs.ZalandoV1().RouteGroups(ns).Create(context.TODO(), rg, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		_, err = waitForRouteGroup(cs, rgCreate.Name, rgCreate.Namespace, 10*time.Minute)
		Expect(err).NotTo(HaveOccurred())
		rgGot, err := cs.ZalandoV1().RouteGroups(ns).Get(context.TODO(), rg.Name, metav1.GetOptions{ResourceVersion: "0"})
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("ALB endpoint from routegroup status: %s", rgGot.Status.LoadBalancer.RouteGroup[0].Hostname))

		// DNS ready
		By("Waiting for ALB, DNS and skipper route to service and pod works")
		err = waitForResponse(hostName, "https", 10*time.Minute, isNotFound, false)
		Expect(err).NotTo(HaveOccurred())

		// checking backend route with predicates
		By("checking the response for a request to /backend we know if we got the correct route")
		err = waitForResponse("https://"+hostName+"/backend", "https", 10*time.Minute, isNotFound, false)
		Expect(err).NotTo(HaveOccurred())
		By("checking the response for a request with headers to /backend we know if we got the correct route")
		req, err := http.NewRequest("GET", "https://"+hostName+"/backend", nil)
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("Foo", "bar")
		resp, err = waitForResponseReturnResponse(req, 10*time.Minute, isSuccess, false)
		Expect(err).NotTo(HaveOccurred())
		s, err := getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Equal(expectedResponse))
	})

	It("Should create routes with filters, predicates and shunt backend [SANDOR] [RouteGroup] [Zalando]", func() {
		var resp *http.Response
		serviceName := "rg-test-fp"
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
		_, err := cs.CoreV1().Services(ns).Create(context.TODO(), service, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		// POD
		By("Creating a POD with prefix " + nameprefix + " in namespace " + ns)
		expectedResponse := "OK RG fp"
		pod := createSkipperPod(
			nameprefix,
			ns,
			fmt.Sprintf(`rHealth: Path("/") -> inlineContent("OK") -> <shunt>;
rBackend: Path("/backend") -> inlineContent("%s") -> <shunt>;
rBackend2: Path("/no-match") -> inlineContent("NOT OK") -> <shunt>;
rBackend3: Path("/multi-methods") -> inlineContent("OK") -> <shunt>;
rBackend4: Path("/router-response") -> inlineContent("NOT OK") -> <shunt>;
`, expectedResponse),
			labels,
			targetPort)

		_, err = cs.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		framework.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(f.ClientSet, pod.Name, pod.Namespace))

		// RouteGroup
		By("Creating a routegroup with name " + serviceName + " in namespace " + ns + " with hostname " + hostName)
		rg := createRouteGroup(serviceName, hostName, ns, labels, nil, port, rgv1.RouteGroupRouteSpec{
			PathSubtree: "/backend",
			Methods:     []rgv1.HTTPMethod{rgv1.MethodGet},
			Predicates: []string{
				`Header("Foo", "bar")`,
			},
			Filters: []string{
				`status(201)`,
			},
		}, rgv1.RouteGroupRouteSpec{
			PathSubtree: "/no-match1",
			Predicates:  []string{`Method("HEAD")`},
		}, rgv1.RouteGroupRouteSpec{
			PathSubtree: "/no-match2",
			Methods:     []rgv1.HTTPMethod{rgv1.MethodHead},
		}, rgv1.RouteGroupRouteSpec{
			PathSubtree: "/multi-methods",
			Methods:     []rgv1.HTTPMethod{rgv1.MethodGet, rgv1.MethodHead},
		}, rgv1.RouteGroupRouteSpec{
			PathSubtree: "/router-response",
			Filters: []string{
				`status(418) -> inlineContent("I am a teapot")`,
			},
			Backends: []rgv1.RouteGroupBackendReference{
				{
					BackendName: "router",
					Weight:      1,
				},
			},
		})
		rgCreate, err := cs.ZalandoV1().RouteGroups(ns).Create(context.TODO(), rg, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		_, err = waitForRouteGroup(cs, rgCreate.Name, rgCreate.Namespace, 10*time.Minute)
		Expect(err).NotTo(HaveOccurred())
		rgGot, err := cs.ZalandoV1().RouteGroups(ns).Get(context.TODO(), rg.Name, metav1.GetOptions{ResourceVersion: "0"})
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("ALB endpoint from routegroup status: %s", rgGot.Status.LoadBalancer.RouteGroup[0].Hostname))

		// DNS ready
		By("Waiting for ALB, DNS and skipper route to service and pod works")
		err = waitForResponse(hostName+"/", "https", 10*time.Minute, isNotFound, false)
		Expect(err).NotTo(HaveOccurred())

		// response for / is from our backend
		By("checking the response code of a request without required request header, we can check if predicate match works correctly")
		req, err := http.NewRequest("GET", "https://"+hostName+"/backend", nil)
		resp, err = waitForResponseReturnResponse(req, 10*time.Minute, isNotFound, false)
		Expect(err).NotTo(HaveOccurred())
		resp.Body.Close()

		// checking backend route with predicates and filters
		By("checking the response status code for a request to /backend without correct headers we should get 404")
		err = waitForResponse("https://"+hostName+"/backend", "https", 10*time.Minute, isNotFound, false)
		By("checking the response for a request to /backend with the right header we know if we got the correct route")
		req, err = http.NewRequest("GET", "https://"+hostName+"/backend", nil)
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("Foo", "bar")
		resp, err = waitForResponseReturnResponse(req, 10*time.Minute, func(code int) bool {
			return code == http.StatusCreated
		}, false)
		Expect(err).NotTo(HaveOccurred())
		s, err := getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Equal(expectedResponse))

		By("checking /no-match1 unexpected method should lead to 404")
		err = waitForResponse("https://"+hostName+"/no-match1", "https", 10*time.Minute, isNotFound, false)
		Expect(err).NotTo(HaveOccurred())

		By("checking /no-match2 unexpected predicate should lead to 404")
		err = waitForResponse("https://"+hostName+"/no-match2", "https", 10*time.Minute, isNotFound, false)
		Expect(err).NotTo(HaveOccurred())

		By("checking /multi-methods matches correctly")
		req, err = http.NewRequest("GET", "https://"+hostName+"/multi-methods", nil)
		Expect(err).NotTo(HaveOccurred())
		resp, err = waitForResponseReturnResponse(req, 10*time.Minute, isSuccess, false)
		Expect(err).NotTo(HaveOccurred())
		resp.Body.Close()
		req, err = http.NewRequest("HEAD", "https://"+hostName+"/multi-methods", nil)
		Expect(err).NotTo(HaveOccurred())
		resp, err = waitForResponseReturnResponse(req, 10*time.Minute, isSuccess, false)
		Expect(err).NotTo(HaveOccurred())
		resp.Body.Close()

		By("checking /router-response matches correctly and response with shunted route")
		err = waitForResponse("https://"+hostName+"/router-response", "https", 10*time.Minute, func(code int) bool {
			return code == http.StatusTeapot
		}, false)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should create blue-green routes [RouteGroup] [Zalando]", func() {
		var resp *http.Response
		serviceName := "rg-test-fp"
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
		_, err := cs.CoreV1().Services(ns).Create(context.TODO(), service, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		// POD
		By("Creating a POD with prefix " + nameprefix + " in namespace " + ns)
		expectedResponse := "OK RG fp"
		pod := createSkipperPod(
			nameprefix,
			ns,
			fmt.Sprintf(`rHealth: Path("/") -> inlineContent("OK") -> <shunt>;
rBackend: Path("/backend") -> inlineContent("%s") -> <shunt>;`,
				expectedResponse),
			labels,
			targetPort)

		_, err = cs.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		framework.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(f.ClientSet, pod.Name, pod.Namespace))

		// RouteGroup
		By("Creating a routegroup with name " + serviceName + " in namespace " + ns + " with hostname " + hostName)
		rg := createRouteGroup(serviceName, hostName, ns, labels, nil, port, rgv1.RouteGroupRouteSpec{
			PathSubtree: "/",
		}, rgv1.RouteGroupRouteSpec{
			PathSubtree: "/blue-green",
			Filters: []string{
				`status(201) -> inlineContent("blue")`,
			},
			Backends: []rgv1.RouteGroupBackendReference{
				{
					BackendName: "router",
					Weight:      1,
				},
			},
		}, rgv1.RouteGroupRouteSpec{
			PathSubtree: "/blue-green",
			Predicates: []string{
				`Traffic(0.5)`,
			},
			Filters: []string{
				`status(202) -> inlineContent("green")`,
			},
			Backends: []rgv1.RouteGroupBackendReference{
				{
					BackendName: "router",
					Weight:      1,
				},
			},
		})
		rgCreate, err := cs.ZalandoV1().RouteGroups(ns).Create(context.TODO(), rg, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		_, err = waitForRouteGroup(cs, rgCreate.Name, rgCreate.Namespace, 10*time.Minute)
		Expect(err).NotTo(HaveOccurred())
		rgGot, err := cs.ZalandoV1().RouteGroups(ns).Get(context.TODO(), rg.Name, metav1.GetOptions{ResourceVersion: "0"})
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("ALB endpoint from routegroup status: %s", rgGot.Status.LoadBalancer.RouteGroup[0].Hostname))

		// DNS ready
		By("Waiting for ALB, DNS and skipper route to service and pod works")
		err = waitForResponse(hostName, "https", 10*time.Minute, isSuccess, false)
		Expect(err).NotTo(HaveOccurred())

		// response for / is from our backend
		By("checking the response body we know, if we got the response from our backend")
		req, err := http.NewRequest("GET", "https://"+hostName+"/", nil)
		Expect(err).NotTo(HaveOccurred())
		resp, err = waitForResponseReturnResponse(req, 10*time.Minute, func(code int) bool {
			return code == 200
		}, false)
		Expect(err).NotTo(HaveOccurred())
		s, err := getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Equal("OK"))

		// checking blue-green routes are ~50/50 match
		By("checking the response for a request to /blue-green we know if we got the correct route")
		req, err = http.NewRequest("GET", "https://"+hostName+"/blue-green", nil)
		Expect(err).NotTo(HaveOccurred())
		resp, err = waitForResponseReturnResponse(req, 10*time.Minute, func(code int) bool {
			return code > 200 && code < 203
		}, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Or(Equal(201), Equal(202)))
		resp.Body.Close()

		cnt := map[int]int{
			201: 0,
			202: 0,
		}
		for i := 0; i < 100; i++ {
			resp, err = waitForResponseReturnResponse(req, 10*time.Minute, func(code int) bool {
				return code > 200 && code < 203
			}, false)
			Expect(err).NotTo(HaveOccurred())
			resp.Body.Close()
			cnt[resp.StatusCode]++
		}
		res201 := cnt[201] > 40 && cnt[201] < 60
		res202 := cnt[202] > 40 && cnt[202] < 60
		Expect(res201).To(BeTrue())
		Expect(res202).To(BeTrue())
	})

	It("Should create gradual traffic routes [RouteGroup] [Zalando]", func() {
		var resp *http.Response
		serviceName := "rg-blue"
		serviceName2 := "rg-green"
		nameprefix := serviceName + "-"
		nameprefix2 := serviceName2 + "-"
		ns := f.Namespace.Name
		hostName := fmt.Sprintf("%s-%d.%s", serviceName, time.Now().UTC().Unix(), E2EHostedZone())
		labels := map[string]string{
			"app": serviceName,
		}
		labels2 := map[string]string{
			"app": serviceName2,
		}
		port := 83
		targetPort := 80

		// SVC
		By("Creating service " + serviceName + " in namespace " + ns)
		service := createServiceTypeClusterIP(serviceName, labels, port, targetPort)
		By("Creating service2 " + serviceName2 + " in namespace " + ns)
		service2 := createServiceTypeClusterIP(serviceName2, labels2, port, targetPort)
		_, err := cs.CoreV1().Services(ns).Create(context.TODO(), service, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		_, err = cs.CoreV1().Services(ns).Create(context.TODO(), service2, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		// POD
		By("Creating 2 PODs with prefix " + nameprefix + " and " + nameprefix + " in namespace " + ns)
		expectedResponse := "blue"
		pod := createSkipperPod(
			nameprefix,
			ns,
			fmt.Sprintf(`rHealth: Path("/") -> inlineContent("OK") -> <shunt>;
rBackend: Path("/blue-green") -> status(201) -> inlineContent("%s") -> <shunt>;`,
				expectedResponse),
			labels,
			targetPort)

		expectedResponse2 := "green"
		pod2 := createSkipperPod(
			nameprefix2,
			ns,
			fmt.Sprintf(`rHealth: Path("/") -> inlineContent("OK") -> <shunt>;
rBackend: Path("/blue-green") -> status(202) -> inlineContent("%s") -> <shunt>;`,
				expectedResponse2),
			labels2,
			targetPort)

		_, err = cs.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		_, err = cs.CoreV1().Pods(ns).Create(context.TODO(), pod2, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		framework.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(f.ClientSet, pod.Name, pod.Namespace))
		framework.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(f.ClientSet, pod2.Name, pod2.Namespace))

		// RouteGroup
		By("Creating a routegroup with name " + serviceName + "-" + serviceName2 + " in namespace " + ns + " with hostname " + hostName)
		rg := createRouteGroupWithBackends(serviceName+"-"+serviceName2, hostName, ns, labels, nil, port,
			[]rgv1.RouteGroupBackend{
				{
					Name:        expectedResponse,
					Type:        rgv1.ServiceRouteGroupBackend,
					ServiceName: serviceName,
					ServicePort: port,
				},
				{
					Name:        expectedResponse2,
					Type:        rgv1.ServiceRouteGroupBackend,
					ServiceName: serviceName2,
					ServicePort: port,
				},
				{
					Name: "router",
					Type: rgv1.ShuntRouteGroupBackend,
				},
			}, rgv1.RouteGroupRouteSpec{
				Path: "/",
				Backends: []rgv1.RouteGroupBackendReference{
					{
						BackendName: "router",
					},
				},
				Filters: []string{
					"status(200)",
					`inlineContent("OK")`,
				},
			}, rgv1.RouteGroupRouteSpec{
				PathSubtree: "/blue-green",
				Backends: []rgv1.RouteGroupBackendReference{
					{
						BackendName: expectedResponse,
						Weight:      80,
					},
					{
						BackendName: expectedResponse2,
						Weight:      20,
					},
				},
			})
		rgCreate, err := cs.ZalandoV1().RouteGroups(ns).Create(context.TODO(), rg, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		_, err = waitForRouteGroup(cs, rgCreate.Name, rgCreate.Namespace, 10*time.Minute)
		Expect(err).NotTo(HaveOccurred())
		rgGot, err := cs.ZalandoV1().RouteGroups(ns).Get(context.TODO(), rg.Name, metav1.GetOptions{ResourceVersion: "0"})
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("ALB endpoint from routegroup status: %s", rgGot.Status.LoadBalancer.RouteGroup[0].Hostname))

		// DNS and backend to /blue-green ready
		By("Waiting for ALB, DNS and skipper route to service and pod works")
		req, err := http.NewRequest("GET", "https://"+hostName+"/blue-green", nil)
		Expect(err).NotTo(HaveOccurred())
		resp, err = waitForResponseReturnResponse(req, 10*time.Minute, func(code int) bool {
			return code > 200 && code < 203
		}, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Or(Equal(201), Equal(202)))
		s, err := getBody(resp)
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Or(Equal("blue"), Equal("green")))

		// checking blue-green routes are ~80/20 match
		By("checking the response for a request to /blue-green we know if we got the correct weights for our backends")
		req, err = http.NewRequest("GET", "https://"+hostName+"/blue-green", nil)
		Expect(err).NotTo(HaveOccurred())
		cnt := map[int]int{
			201: 0,
			202: 0,
		}
		for i := 0; i < 100; i++ {
			resp, err = waitForResponseReturnResponse(req, 10*time.Minute, func(code int) bool {
				return code > 200 && code < 203
			}, false)
			Expect(err).NotTo(HaveOccurred())
			resp.Body.Close()
			cnt[resp.StatusCode]++
		}
		// +/- 5 for 80/20
		res201 := cnt[201] > 75 && cnt[201] < 85
		res202 := cnt[202] > 15 && cnt[202] < 25
		Expect(res201).To(BeTrue())
		Expect(res202).To(BeTrue())
	})

	It("Should create NLB routegroup [RouteGroup] [Zalando]", func() {
		serviceName := "rg-test-nlb"
		nameprefix := serviceName + "-"
		ns := f.Namespace.Name
		hostName := fmt.Sprintf("%s-%d.%s", serviceName, time.Now().UTC().Unix(), E2EHostedZone())
		labels := map[string]string{
			"app": serviceName,
		}
		annotations := map[string]string{
			"zalando.org/aws-load-balancer-type": "nlb",
		}
		port := 83
		targetPort := 80
		// SVC
		By("Creating service " + serviceName + " in namespace " + ns)
		service := createServiceTypeClusterIP(serviceName, labels, port, targetPort)
		_, err := cs.CoreV1().Services(ns).Create(context.TODO(), service, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		// POD
		By("Creating a POD with prefix " + nameprefix + " in namespace " + ns)
		pod := createSkipperPod(
			nameprefix,
			ns,
			`rHealth: Path("/") -> inlineContent("OK") -> <shunt>`,
			labels,
			targetPort)

		_, err = cs.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		framework.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(f.ClientSet, pod.Name, pod.Namespace))

		// RouteGroup
		By("Creating a routegroup with name " + serviceName + " in namespace " + ns + " with hostname " + hostName)
		rg := createRouteGroup(serviceName, hostName, ns, labels, annotations, port, rgv1.RouteGroupRouteSpec{
			PathSubtree: "/",
		})
		rgCreate, err := cs.ZalandoV1().RouteGroups(ns).Create(context.TODO(), rg, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		_, err = waitForRouteGroup(cs, rgCreate.Name, rgCreate.Namespace, 10*time.Minute)
		Expect(err).NotTo(HaveOccurred())
		rgGot, err := cs.ZalandoV1().RouteGroups(ns).Get(context.TODO(), rg.Name, metav1.GetOptions{ResourceVersion: "0"})
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("NLB endpoint from routegroup status: %s", rgGot.Status.LoadBalancer.RouteGroup[0].Hostname))

		// DNS ready
		By("Waiting for NLB, DNS and skipper route to service and pod works")
		err = waitForResponse(hostName, "https", 10*time.Minute, isSuccess, false)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should create ALB routegroup with 2 hostnames [RouteGroup] [Zalando]", func() {
		serviceName := "rg-test-2hosts"
		nameprefix := serviceName + "-"
		ns := f.Namespace.Name
		hostName := fmt.Sprintf("%s-%d.%s", serviceName, time.Now().UTC().Unix(), E2EHostedZone())
		hostName2 := fmt.Sprintf("%s-2-%d.%s", serviceName, time.Now().UTC().Unix(), E2EHostedZone())
		labels := map[string]string{
			"app": serviceName,
		}
		port := 83
		targetPort := 80
		// SVC
		By("Creating service " + serviceName + " in namespace " + ns)
		service := createServiceTypeClusterIP(serviceName, labels, port, targetPort)
		_, err := cs.CoreV1().Services(ns).Create(context.TODO(), service, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		// POD
		By("Creating a POD with prefix " + nameprefix + " in namespace " + ns)
		pod := createSkipperPod(
			nameprefix,
			ns,
			`rHealth: Path("/") -> inlineContent("OK") -> <shunt>`,
			labels,
			targetPort)

		_, err = cs.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		framework.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(f.ClientSet, pod.Name, pod.Namespace))

		// RouteGroup
		By("Creating a routegroup with name " + serviceName + " in namespace " + ns + " with hostname " + hostName)
		rg := createRouteGroup(serviceName, hostName, ns, labels, nil, port, rgv1.RouteGroupRouteSpec{
			PathSubtree: "/",
		})
		rg.Spec.Hosts = append(rg.Spec.Hosts, hostName2) // add second hostname
		rgCreate, err := cs.ZalandoV1().RouteGroups(ns).Create(context.TODO(), rg, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		_, err = waitForRouteGroup(cs, rgCreate.Name, rgCreate.Namespace, 10*time.Minute)
		Expect(err).NotTo(HaveOccurred())
		rgGot, err := cs.ZalandoV1().RouteGroups(ns).Get(context.TODO(), rg.Name, metav1.GetOptions{ResourceVersion: "0"})
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("ALB endpoint from routegroup status: %s", rgGot.Status.LoadBalancer.RouteGroup[0].Hostname))

		// DNS ready for both endpoints
		By("Waiting for ALB, DNS and skipper route to service and pod works")
		err = waitForResponse(hostName, "https", 10*time.Minute, isSuccess, false)
		Expect(err).NotTo(HaveOccurred())
		err = waitForResponse(hostName2, "https", 10*time.Minute, isSuccess, false)
		Expect(err).NotTo(HaveOccurred())
	})

})
