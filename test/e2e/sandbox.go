/*
Copyright 2024 The Kubernetes Authors.
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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	rgv1 "github.com/szuecs/routegroup-client/apis/zalando.org/v1"
	sandboxv1 "github.com/zalando-build/sandbox-controller/pkg/apis/zalando.org/v1"
	"github.com/zalando-build/sandbox-controller/pkg/clientset"
)

func waitForSandboxedRoutegrouop(ctx context.Context, c clientset.Interface, ns, originalRGName string) (*rgv1.RouteGroup, error) {
	var sandboxedRG *rgv1.RouteGroup
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 1*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		rgs, err := c.ZalandoV1().RouteGroups(ns).List(
			context.TODO(),
			metav1.ListOptions{
				LabelSelector: "zalando.org/sandbox-original-routegroup=" + originalRGName,
			},
		)
		if err != nil {
			return true, err
		}
		if len(rgs.Items) > 0 {
			sandboxedRG = &rgs.Items[0]
			return true, nil
		}
		return false, nil
	})
	return sandboxedRG, err
}

func waitForSandboxedIngress(ctx context.Context, c clientset.Interface, ns, originalIngName string) (*netv1.Ingress, error) {
	var sandboxedIng *netv1.Ingress
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 1*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		ings, err := c.NetworkingV1().Ingresses(ns).List(
			context.TODO(),
			metav1.ListOptions{
				LabelSelector: "zalando.org/sandbox-original-ingress=" + originalIngName,
			},
		)
		if err != nil {
			return true, err
		}
		if len(ings.Items) > 0 {
			sandboxedIng = &ings.Items[0]
			return true, nil
		}
		return false, nil
	})
	return sandboxedIng, err
}

func createSandbox(name, ns string, labels map[string]string, annotations map[string]string, sources []string, target string) *sandboxv1.Sandbox {
	return &sandboxv1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name + "-" + string(uuid.NewUUID()),
			Namespace:   ns,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: sandboxv1.SandboxSpec{
			SourceHosts: sources,
			Target:      target,
			TestProject: "test-project-" + string(uuid.NewUUID()),
		},
	}
}

var _ = describe("Sandbox Controller", func() {
	f := framework.NewDefaultFramework("sandbox-controller")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelBaseline

	var (
		c clientset.Interface
	)

	BeforeEach(func() {
		var err error

		config, err := framework.LoadConfig()
		framework.ExpectNoError(err)

		c, err = clientset.NewForConfig(config)
		framework.ExpectNoError(err)
	})

	Describe("Sandbox resource", func() {
		It("Should process a Sandbox resource and create a routegroup [Sandbox] [Zalando]", func() {
			ns := f.Namespace.Name
			app := "sandbox-test"
			labels := map[string]string{
				"app": app,
			}
			sandbox := createSandbox(app, ns, labels, nil, []string{"example.org"}, "https://sandbox.example.org")
			rg := createRouteGroup(app, "example.org", ns, labels, nil, 9090, rgv1.RouteGroupRouteSpec{
				PathSubtree: "/",
				Methods:     []rgv1.HTTPMethod{rgv1.MethodGet},
				Predicates:  []string{`Header("Foo", "bar")`},
			})

			By(fmt.Sprintf("Creating Sandbox %s in namespace %s", sandbox.Name, ns))
			sbCreate, err := c.ZalandoV1().Sandboxes(ns).Create(context.TODO(), sandbox, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			Expect(sbCreate.Name).To(Equal(sandbox.Name))

			By(fmt.Sprintf("Creating RouteGroup %s in namespace %s", rg.GetName(), ns))
			rgCreate, err := c.ZalandoV1().RouteGroups(ns).Create(context.TODO(), rg, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			Expect(rgCreate.Name).To(Equal(rg.Name))

			By("Verifying Sandboxed RouteGroup was created successfully")
			sandboxedRG, err := waitForSandboxedRoutegrouop(context.TODO(), c, ns, rgCreate.Name)
			framework.ExpectNoError(err)
			Expect(sandboxedRG).NotTo(BeNil())

			predicates := sandboxedRG.Spec.Routes[0].Predicates
			Expect(predicates).To(ContainElement(`HeaderRegexp("X-Zalando-Client-Id", "^test:` + sandbox.Spec.TestProject + `:.*")`))
			Expect(predicates).To(ContainElement(`Header("Foo", "bar")`))

			backends := sandboxedRG.Spec.Backends
			Expect(backends).To(HaveLen(1))
			Expect(backends[0].Address).To(Equal("https://sandbox.example.org"))

			By("Deleting the Sandbox resource")
			err = c.ZalandoV1().Sandboxes(ns).Delete(context.TODO(), sandbox.Name, metav1.DeleteOptions{})
			framework.ExpectNoError(err)

			By("Verifying SandboxedRouteDroup is deleted")
			Eventually(func() ([]rgv1.RouteGroup, error) {
				r, err := c.ZalandoV1().RouteGroups(ns).List(
					context.TODO(),
					metav1.ListOptions{
						LabelSelector: "zalando.org/sandbox-original-routegroup=" + rgCreate.Name,
					},
				)
				if err != nil {
					return nil, err
				}
				return r.Items, nil
			}).WithTimeout(1 * time.Minute).WithPolling(1 * time.Second).Should(HaveLen(0))
		})

		It("Should proocess a Sandbox resource and create an ingress [Sandbox] [Zalando]", func() {
			ns := f.Namespace.Name
			app := "sandbox-ingress-test"
			labels := map[string]string{
				"app": app,
			}
			sandbox := createSandbox(app, ns, labels, nil, []string{"example.com"}, "https://sandbox.example.com")
			ing := createIngress(app, "example.com", ns, "/", netv1.PathTypeImplementationSpecific, labels, nil, 8080)

			By(fmt.Sprintf("Creating Sandbox %s in namespace %s", sandbox.Name, ns))
			sbCreate, err := c.ZalandoV1().Sandboxes(ns).Create(context.TODO(), sandbox, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			Expect(sbCreate.Name).To(Equal(sandbox.Name))

			By(fmt.Sprintf("Creating Ingress %s in namespace %s", ing.GetName(), ns))
			ingCreate, err := c.NetworkingV1().Ingresses(ns).Create(context.TODO(), ing, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			Expect(ingCreate.Name).To(Equal(ing.Name))

			By("Verifying Sandboxed Ingress was created successfully")
			sandboxedIng, err := waitForSandboxedIngress(context.TODO(), c, ns, ingCreate.Name)
			framework.ExpectNoError(err)
			Expect(sandboxedIng).NotTo(BeNil())

			By("Deleting the Origignal Ingress resource")
			err = c.NetworkingV1().Ingresses(ns).Delete(context.TODO(), ing.Name, metav1.DeleteOptions{})
			framework.ExpectNoError(err)

			By("Verifying Sandboxed Ingress is deleted")
			Eventually(func() ([]netv1.Ingress, error) {
				i, err := c.NetworkingV1().Ingresses(ns).List(
					context.TODO(),
					metav1.ListOptions{
						LabelSelector: "zalando.org/sandbox-original-ingress=" + ingCreate.Name,
					},
				)
				if err != nil {
					return nil, err
				}
				return i.Items, nil
			}).WithTimeout(1 * time.Minute).WithPolling(1 * time.Second).Should(HaveLen(0))
		})

	})

	Describe("SandboxEgress resource", func() {
		It("Should intercept traffic with SandboxEgress and mock responses [SandboxEgress] [Zalando]", func() {
			ns := f.Namespace.Name
			labels := map[string]string{"app": "production-backend"}

			// Step 1: Create a simple production backend pod that serves responses
			By("Creating production backend pod")

			route := `* -> inlineContent("production backend pod") -> <shunt>`
			backendPod := createSkipperPod("production-backend-", ns, route, labels, 9990)
			_, err := c.CoreV1().Pods(ns).Create(context.TODO(), backendPod, metav1.CreateOptions{})
			framework.ExpectNoError(err, "failed to create pod: %s in namespace: %s", backendPod.Name, ns)

			By("Waiting for production backend pod to be running")
			framework.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(context.TODO(), f.ClientSet, backendPod.Name, ns), "failed to wait for pod: %s in namespace: %s", backendPod.Name, ns)

			By("Finding production backend pod IP")
			// createdBackendPod, err := c.CoreV1().Pods(ns).Get(context.TODO(), backendPod.Name, metav1.GetOptions{})
			// backendUrl := fmt.Sprintf("http://%s:9990", createdBackendPod.Status.PodIP)

			productionService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "production-backend",
					Namespace: ns,
					Labels:    labels,
				},
				Spec: corev1.ServiceSpec{
					Selector: labels,
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(9990),
							Name:       "http",
						},
					},
				},
			}

			_, err = c.CoreV1().Services(ns).Create(context.TODO(), productionService, metav1.CreateOptions{})
			framework.ExpectNoError(err)

			By("Creating ConfigMap for egress routes")
			configMapName := "test-egress-routes-" + string(uuid.NewUUID())
			initialRoutes := `
        catchAllLocal: Host(".*[.]cluster[.]local$") -> <dynamic>;
        catchAll: * -> setDynamicBackendScheme("https") -> <dynamic>;
      `
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: ns,
					Labels:    map[string]string{"app": "test-egress-app"},
				},
				Data: map[string]string{
					"routes.eskip": initialRoutes,
				},
			}
			_, err = c.CoreV1().ConfigMaps(ns).Create(context.TODO(), cm, metav1.CreateOptions{})
			framework.ExpectNoError(err)

			By("Creating egress-ready pod with Skipper sidecar")
			egressReadyPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-egress-pod-" + string(uuid.NewUUID()),
					Namespace: ns,
					Labels:    map[string]string{"app": "test-egress-app"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "app",
							Image:   "curlimages/curl:latest",
							Command: []string{"sleep"},
							Args:    []string{"3600"},
							Env: []corev1.EnvVar{
								{
									Name:  "http_proxy", // curl requires lowercase
									Value: "http://localhost:9090",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
							},
						},
						{
							Name:  "egress-proxy",
							Image: "registry.opensource.zalan.do/teapot/skipper:latest",
							Ports: []corev1.ContainerPort{
								{ContainerPort: 9090, Name: "egress"},
							},
							Args: []string{
								"skipper",
								"-address=:9090",
								"-routes-file=/config/routes.eskip",
								"-wait-first-route-load",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "egress-config",
									MountPath: "/config",
									ReadOnly:  true,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("100Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("100Mi"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "egress-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapName,
									},
								},
							},
						},
					},
				},
			}
			createdPod, err := c.CoreV1().Pods(ns).Create(context.TODO(), egressReadyPod, metav1.CreateOptions{})
			framework.ExpectNoError(err)

			By("Waiting for egress-ready pod to be running")
			err = wait.PollUntilContextTimeout(context.TODO(), 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
				pod, err := c.CoreV1().Pods(ns).Get(context.TODO(), createdPod.Name, metav1.GetOptions{})
				if err != nil {
					return false, nil
				}
				if pod.Status.Phase == corev1.PodRunning {
					return true, nil
				}
				return false, nil
			})
			framework.ExpectNoError(err)

			By("Executing HTTP request to production backend (should reach production)")
			testProject := "test-project-" + string(uuid.NewUUID())
			productionURL := fmt.Sprintf("http://production-backend.%s.svc.cluster.local", ns)
			cmd := fmt.Sprintf(`curl -s -H "X-Zalando-Client-Id: test:%s:dummy" %s`, testProject, productionURL)

			output, err := e2ekubectl.RunKubectl(ns, "exec", createdPod.Name, "-c", "app", "--", "sh", "-c", cmd)
			framework.ExpectNoError(err)
			framework.Logf("Production backend response: %s", output)
			Expect(output).To(ContainSubstring("production"))

			By("Creating SandboxEgress resource to mock responses")
			mockRoutes := []sandboxv1.Route{
				{
					Host: "production-backend." + ns + ".svc.cluster.local",
					Hostroutes: []sandboxv1.HostRoute{
						{
							Path: "/",
							PathRoutes: []sandboxv1.PathRoute{
								{
									Methods: []sandboxv1.HTTPMethod{
										sandboxv1.HTTPMethod("GET"),
										sandboxv1.HTTPMethod("POST"),
									},
									EndpointRoute: sandboxv1.EndpointRoute{
										Body:   string("intercepted response"),
										Status: 200,
									},
								},
							},
						},
					},
				},
			}

			se := &sandboxv1.SandboxEgress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-egress-mock-" + string(uuid.NewUUID()),
					Namespace: ns,
					Labels:    map[string]string{"app": "test-egress-app"},
				},
				Spec: sandboxv1.SandboxEgressSpec{
					TestProject:  testProject,
					ConfigMapRef: configMapName,
					Routes:       mockRoutes,
				},
			}

			_, err = c.ZalandoV1().SandboxEgresses(ns).Create(context.TODO(), se, metav1.CreateOptions{})
			framework.ExpectNoError(err)

			By("Waiting for SandboxEgress to be processed and routes to be updated in ConfigMap")
			err = wait.PollUntilContextTimeout(context.TODO(), 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
				cm, err := c.CoreV1().ConfigMaps(ns).Get(context.TODO(), configMapName, metav1.GetOptions{})
				if err != nil {
					return false, nil
				}
				routes, exists := cm.Data["routes.eskip"]
				if !exists {
					return false, nil
				}
				if routes != initialRoutes {
					return true, nil
				}
				return false, nil
			})

			time.Sleep(2 * time.Second) // small delay to ensure Skipper picks up the new routes

			By("Executing HTTP request to verify mocked response is returned")
			output, err = e2ekubectl.RunKubectl(ns, "exec", createdPod.Name, "-c", "app", "--", "sh", "-c", cmd)
			framework.ExpectNoError(err)
			framework.Logf("Mocked response: %s", output)
			Expect(output).To(Equal("intercepted response"))
		})
	})

})
