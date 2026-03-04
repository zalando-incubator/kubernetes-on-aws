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

	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = describe("Ingress Validation Webhook", func() {
	f := framework.NewDefaultFramework("skipper-ingress-validation")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelBaseline
	var (
		cs kubernetes.Interface
	)
	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("Should reject ingress with invalid skipper annotations [Validation]", func() {
		serviceName := "test-validation-service"
		ns := f.Namespace.Name

		By("Testing invalid predicate annotation")
		ing := createInvalidPredicateIngress(serviceName, ns)
		_, err := cs.NetworkingV1().Ingresses(ns).Create(context.TODO(), ing, metav1.CreateOptions{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("admission webhook"))

		By("Testing invalid filter annotation")
		ing2 := createInvalidFilterIngress(serviceName, ns)
		_, err = cs.NetworkingV1().Ingresses(ns).Create(context.TODO(), ing2, metav1.CreateOptions{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("admission webhook"))
	})

	It("Should reject ingress update with invalid annotations [Validation]", func() {
		serviceName := "test-update-validation"
		ns := f.Namespace.Name

		By("Creating a valid ingress first")
		ing := createIngress(serviceName, fmt.Sprintf("%s.example.com", serviceName), ns, "/", netv1.PathTypeImplementationSpecific, map[string]string{"app": serviceName}, map[string]string{"zalando.org/skipper-predicate": "Method(\"GET\")"}, 80)
		created, err := cs.NetworkingV1().Ingresses(ns).Create(context.TODO(), ing, metav1.CreateOptions{})
		framework.ExpectNoError(err)

		defer func() {
			err := cs.NetworkingV1().Ingresses(ns).Delete(context.TODO(), created.Name, metav1.DeleteOptions{})
			framework.ExpectNoError(err)
		}()

		By("Attempting to update with invalid predicate")
		created.Annotations["zalando.org/skipper-predicate"] = "sssssstatusssss()"
		_, err = cs.NetworkingV1().Ingresses(ns).Update(context.TODO(), created, metav1.UpdateOptions{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("admission webhook"))
	})
})

func createInvalidPredicateIngress(name, namespace string) *netv1.Ingress {
	return &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-invalid-predicate", name),
			Namespace: namespace,
			Annotations: map[string]string{
				"zalando.org/skipper-predicate": "NonExistingPredicate(\"foo\")",
			},
		},
		Spec: netv1.IngressSpec{
			Rules: []netv1.IngressRule{
				{
					Host: fmt.Sprintf("%s.example.com", name),
					IngressRuleValue: netv1.IngressRuleValue{
						HTTP: &netv1.HTTPIngressRuleValue{
							Paths: []netv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &[]netv1.PathType{netv1.PathTypeImplementationSpecific}[0],
									Backend: netv1.IngressBackend{
										Service: &netv1.IngressServiceBackend{
											Name: name,
											Port: netv1.ServiceBackendPort{Number: 80},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func createInvalidFilterIngress(name, namespace string) *netv1.Ingress {
	return &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-invalid-filter", name),
			Namespace: namespace,
			Annotations: map[string]string{
				"zalando.org/skipper-filter": "undefinedFilter()",
			},
		},
		Spec: netv1.IngressSpec{
			Rules: []netv1.IngressRule{
				{
					Host: fmt.Sprintf("%s.example.com", name),
					IngressRuleValue: netv1.IngressRuleValue{
						HTTP: &netv1.HTTPIngressRuleValue{
							Paths: []netv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &[]netv1.PathType{netv1.PathTypeImplementationSpecific}[0],
									Backend: netv1.IngressBackend{
										Service: &netv1.IngressServiceBackend{
											Name: name,
											Port: netv1.ServiceBackendPort{Number: 80},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
