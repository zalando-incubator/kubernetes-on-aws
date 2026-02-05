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

	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
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

})
