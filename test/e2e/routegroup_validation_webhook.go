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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rgclient "github.com/szuecs/routegroup-client"
	rgv1 "github.com/szuecs/routegroup-client/apis/zalando.org/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = describe("RouteGroup Validation Webhook", func() {
	f := framework.NewDefaultFramework("skipper-routegroup-validation")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelBaseline
	var (
		rgcs rgclient.Interface
	)
	BeforeEach(func() {
		By("Creating an rgclient Clientset")
		config, err := framework.LoadConfig()
		framework.ExpectNoError(err)
		config.QPS = f.Options.ClientQPS
		config.Burst = f.Options.ClientBurst
		if f.Options.GroupVersion != nil {
			config.GroupVersion = f.Options.GroupVersion
		}
		rgcs, err = rgclient.NewClientset(config)
		framework.ExpectNoError(err)
	})

	It("Should reject routegroup with invalid predicates and filters [Validation]", func() {
		serviceName := "test-rg-validation"
		ns := f.Namespace.Name

		// Test case 1: Invalid predicate syntax
		By("Testing invalid predicate in RouteGroup")
		rg := createInvalidPredicateRouteGroup(serviceName, ns)
		_, err := rgcs.ZalandoV1().RouteGroups(ns).Create(context.TODO(), rg, metav1.CreateOptions{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("admission webhook"))

		// Test case 2: Invalid filter syntax
		By("Testing invalid filter in RouteGroup")
		rg2 := createInvalidFilterRouteGroup(serviceName, ns)
		_, err = rgcs.ZalandoV1().RouteGroups(ns).Create(context.TODO(), rg2, metav1.CreateOptions{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("admission webhook"))
	})

	It("Should reject routegroup update with invalid syntax [Validation]", func() {
		serviceName := "test-rg-update-validation"
		ns := f.Namespace.Name

		By("Creating a valid RouteGroup first")
		rg := createRouteGroup(serviceName, fmt.Sprintf("%s.example.com", serviceName), ns, map[string]string{"app": serviceName}, nil, 80, rgv1.RouteGroupRouteSpec{
			PathSubtree: "/",
		})
		created, err := rgcs.ZalandoV1().RouteGroups(ns).Create(context.TODO(), rg, metav1.CreateOptions{})
		framework.ExpectNoError(err)

		defer func() {
			err := rgcs.ZalandoV1().RouteGroups(ns).Delete(context.TODO(), created.Name, metav1.DeleteOptions{})
			framework.ExpectNoError(err)
		}()

		By("Attempting to update with invalid predicate")
		created.Spec.Routes[0].Predicates = []string{"sssssstatusssss()"}
		_, err = rgcs.ZalandoV1().RouteGroups(ns).Update(context.TODO(), created, metav1.UpdateOptions{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("admission webhook"))
	})
})

// Helper functions to create test routegroups

func createInvalidPredicateRouteGroup(name, namespace string) *rgv1.RouteGroup {
	return &rgv1.RouteGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-invalid-predicate", name),
			Namespace: namespace,
		},
		Spec: rgv1.RouteGroupSpec{
			Hosts: []string{fmt.Sprintf("%s.example.com", name)},
			Backends: []rgv1.RouteGroupBackend{
				{
					Name:        name,
					Type:        rgv1.ServiceRouteGroupBackend,
					ServiceName: name,
					ServicePort: 80,
				},
			},
			Routes: []rgv1.RouteGroupRouteSpec{
				{
					PathSubtree: "/unknown-predicate",
					Predicates:  []string{"NonExistingPredicate(\"foo\")"},
					Filters:     []string{"status(200)"},
					Backends: []rgv1.RouteGroupBackendReference{
						{BackendName: name, Weight: 1},
					},
				},
			},
		},
	}
}

func createInvalidFilterRouteGroup(name, namespace string) *rgv1.RouteGroup {
	return &rgv1.RouteGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-invalid-filter", name),
			Namespace: namespace,
		},
		Spec: rgv1.RouteGroupSpec{
			Hosts: []string{fmt.Sprintf("%s.example.com", name)},
			Backends: []rgv1.RouteGroupBackend{
				{
					Name:        name,
					Type:        rgv1.ServiceRouteGroupBackend,
					ServiceName: name,
					ServicePort: 80,
				},
			},
			Routes: []rgv1.RouteGroupRouteSpec{
				{
					PathSubtree: "/unknown-filter",
					Filters:     []string{"undefinedFilter()", "status(200)"},
					Backends: []rgv1.RouteGroupBackendReference{
						{BackendName: name, Weight: 1},
					},
				},
			},
		},
	}
}
