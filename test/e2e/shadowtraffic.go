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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	rgv1 "github.com/szuecs/routegroup-client/apis/zalando.org/v1"
	"github.com/zalando-build/shadow-traffic-controller/controller"
	stv1 "github.com/zalando-build/shadow-traffic-controller/pkg/apis/zalando.org/v1"
	"github.com/zalando-build/shadow-traffic-controller/pkg/clientset"
)

// waitForShadowRouteGroups polls until the expected number of shadow RouteGroups
// (labelled with ownerLabel=stName) exist in the namespace, then returns them.
func waitForShadowRouteGroups(
	ctx context.Context,
	client *clientset.Clientset,
	namespace, stName string,
	expectedCount int,
) ([]rgv1.RouteGroup, error) {
	var rgs *rgv1.RouteGroupList
	err := wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 30*time.Second, false,
		func(ctx context.Context) (bool, error) {
			var listErr error
			rgs, listErr = client.ZalandoV1().RouteGroups(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", controller.OwnerLabel, stName),
			})
			if listErr != nil {
				return false, listErr
			}
			return len(rgs.Items) == expectedCount, nil
		})
	if err != nil {
		return nil, fmt.Errorf("failed to wait and list shadow RouteGroups: %w", err)
	}
	return rgs.Items, nil
}

// waitForShadowTrafficStatus polls until the ShadowTraffic's status.problems is non-empty.
func waitForShadowTrafficStatus(
	ctx context.Context,
	client *clientset.Clientset,
	namespace, stName string,
) ([]string, error) {
	var problems []string
	err := wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 30*time.Second, false,
		func(ctx context.Context) (bool, error) {
			st, getErr := client.ZalandoV1().ShadowTraffics(namespace).Get(ctx, stName, metav1.GetOptions{})
			if getErr != nil {
				return false, getErr
			}
			problems = st.Status.Problems
			return len(problems) > 0, nil
		})
	if err != nil {
		return nil, fmt.Errorf("failed to wait and get ShadowTraffic status problems: %w", err)
	}
	return problems, nil
}

// waitForNoShadowRouteGroups polls until zero shadow RouteGroups exist for the given owner.
func waitForNoShadowRouteGroups(
	ctx context.Context,
	client *clientset.Clientset,
	namespace, stName string,
) (bool, error) {
	err := wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 30*time.Second, false,
		func(ctx context.Context) (bool, error) {
			rgs, listErr := client.ZalandoV1().RouteGroups(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", controller.OwnerLabel, stName),
			})
			if listErr != nil {
				return false, listErr
			}
			return len(rgs.Items) == 0, nil
		})
	if err != nil {
		return false, fmt.Errorf("failed to wait while no shadow RouteGroups: %w", err)
	}
	return true, nil
}

func createShadowTraffic(
	name string,
	ns string,
	labels map[string]string,
	annotations map[string]string,
	sourceRefs []stv1.SourceObjectReference,
	trafficShare string,
	shadowBackend stv1.ShadowBackend,
	routeMatchers ...stv1.RouteMatcher,
) *stv1.ShadowTraffic {
	return &stv1.ShadowTraffic{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: stv1.ShadowTrafficSpec{
			SourceObjectReferences: sourceRefs,
			TrafficShare:           trafficShare,
			ShadowBackend:          shadowBackend,
			RouteMatchers:          routeMatchers,
		},
	}
}

var _ = describe("Shadow Traffic Controller", func() {
	f := framework.NewDefaultFramework("shadow-traffic-controller")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelBaseline

	var (
		c *clientset.Clientset
	)

	BeforeEach(func() {
		var err error

		config, err := framework.LoadConfig()
		framework.ExpectNoError(err)

		c, err = clientset.NewForConfig(config)
		framework.ExpectNoError(err)
	})

	Describe("ShadowTraffic resource: Plain RouteGroup as Source Route Object.", func() {
		It("Should process a ShadowTraffic resource and create the corresponding shadow RouteGroup [ShadowTraffic] [Zalando]", func() {
			ns := f.Namespace.Name
			application := "kubernetes-on-aws-e2e-test"
			component := "shadow-traffic-test"
			labels := map[string]string{
				"application": application,
				"component":   component,
			}

			refRouteGroup := createRouteGroup(
				component,
				"example.org",
				ns,
				labels,
				nil,
				9090,
				rgv1.RouteGroupRouteSpec{
					PathSubtree: "/",
					Methods:     []rgv1.HTTPMethod{rgv1.MethodGet},
				},
				rgv1.RouteGroupRouteSpec{
					PathSubtree: "/",
					Methods:     []rgv1.HTTPMethod{rgv1.MethodPost},
					Predicates:  []string{`Header("Foo", "bar")`},
				},
				rgv1.RouteGroupRouteSpec{
					Path:       "/healthz",
					Methods:    []rgv1.HTTPMethod{rgv1.MethodGet},
					Predicates: []string{`Header("Foo", "bar")`},
				},
			)

			shadowBackend := stv1.ShadowBackend{
				Name:        "shadow-backend",
				Type:        "service",
				ServicePort: 80,
				ServiceName: "shadow-service",
			}
			shadowtraffic := createShadowTraffic(
				string(uuid.NewUUID()),
				ns,
				labels,
				nil,
				[]stv1.SourceObjectReference{{
					Kind:      rgv1.KindRouteGroup,
					Name:      refRouteGroup.Name,
					Namespace: refRouteGroup.Namespace,
				}},
				"0.5",
				shadowBackend,
				stv1.RouteMatcher{
					Path: stv1.Path{
						Type:  "PathSubtree",
						Value: "/",
					},
				},
				stv1.RouteMatcher{
					Path: stv1.Path{
						Type:  "PathSubtree",
						Value: "/",
					},
					Headers: []stv1.Header{`Header("Foo", "bar")`},
					Methods: []stv1.HTTPMethod{stv1.HTTPMethod(rgv1.MethodPost)},
				},
			)

			By(fmt.Sprintf("Creating reference RouteGroup %s in namespace %s", refRouteGroup.Name, ns))
			refrgCreate, err := c.ZalandoV1().RouteGroups(ns).Create(context.TODO(), refRouteGroup, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			Expect(refrgCreate.Name).To(Equal(refRouteGroup.Name))

			By(fmt.Sprintf("Creating ShadowTraffic %s in namespace %s", shadowtraffic.Name, ns))
			stCreate, err := c.ZalandoV1().ShadowTraffics(ns).Create(context.TODO(), shadowtraffic, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			Expect(stCreate.Name).To(Equal(shadowtraffic.Name))

			By("Verifying shadow RouteGroups are created successfully")
			shadowRGs, err := waitForShadowRouteGroups(context.TODO(), c, ns, shadowtraffic.Name, 1)
			framework.ExpectNoError(err)
			Expect(shadowRGs).NotTo(BeEmpty())

			for _, sRG := range shadowRGs {
				By(fmt.Sprintf("Verifying shadow RouteGroup %s has correct labels", sRG.Name))
				Expect(sRG.Labels[controller.OwnerLabel]).To(Equal(shadowtraffic.Name))
				Expect(sRG.Labels[controller.SourceRefLabel]).To(Equal(shadowtraffic.Spec.SourceObjectReferences[0].Name))

				By(fmt.Sprintf("Verifying shadow RouteGroup %s has correct owner reference", sRG.Name))
				Expect(sRG.OwnerReferences).To(HaveLen(1))
				Expect(sRG.OwnerReferences[0].Kind).To(Equal(stv1.KindShadowTraffic))
				Expect(sRG.OwnerReferences[0].Name).To(Equal(shadowtraffic.Name))

				By(fmt.Sprintf("Verifying shadow RouteGroup %s has correct shadow backend", sRG.Name))
				foundShadowBackend := false
				for _, b := range sRG.Spec.Backends {
					if b.Name == shadowBackend.Name {
						foundShadowBackend = true
						Expect(b.Type).To(Equal(rgv1.RouteGroupBackendType(shadowBackend.Type)))
						Expect(b.ServiceName).To(Equal(shadowBackend.ServiceName))
						Expect(b.ServicePort).To(Equal(shadowBackend.ServicePort))
					}
				}
				Expect(foundShadowBackend).To(BeTrue())

				By(fmt.Sprintf("Verifying shadow RouteGroup %s has routes as per the RouteMatchers", sRG.Name))
				// 2 matching source routes * 2 = 4 shadow routes: teeLoopback + tee pair each
				Expect(sRG.Spec.Routes).To(HaveLen(4))
			}

			By("Deleting the ShadowTraffic resource")
			err = c.ZalandoV1().ShadowTraffics(ns).Delete(context.TODO(), shadowtraffic.Name, metav1.DeleteOptions{})
			framework.ExpectNoError(err)

			By("Verifying ShadowTraffic is deleted")
			success, err := waitForNoShadowRouteGroups(context.TODO(), c, ns, shadowtraffic.Name)
			framework.ExpectNoError(err)
			Expect(success).To(BeTrue())
		})
	})
})
