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

	v1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/ingress"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
	admissionapi "k8s.io/pod-security-admission/api"

	. "github.com/onsi/ginkgo/v2"
)

const (
	externalDNSAnnotation = "external-dns.alpha.kubernetes.io/hostname"
	serviceName           = "external-dns-test"
	timeout               = 10 * time.Minute
)

var _ = describe("External DNS creation", func() {
	f := framework.NewDefaultFramework("external-dns")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelBaseline
	var (
		cs   kubernetes.Interface
		jigs *e2eservice.TestJig
		jigi *ingress.TestJig
	)

	BeforeEach(func() {
		cs = f.ClientSet
		jigs = e2eservice.NewTestJig(cs, f.Namespace.Name, serviceName)
		jigi = ingress.NewIngressTestJig(f.ClientSet)
	})

	f.It("Should create DNS entry via Service and AWS Cloud Provider [Zalando]", f.WithSlow(), func(ctx context.Context) {
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
			framework.ExpectNoError(err, "failed to delete service: %s in namespace: %s", serviceName, ns)
		}()

		hostName := fmt.Sprintf("%s-%d.%s", serviceName, time.Now().UTC().Unix(), E2EHostedZone())
		_, err := jigs.CreateLoadBalancerService(ctx, timeout, func(svc *v1.Service) {
			svc.ObjectMeta = metav1.ObjectMeta{
				Name: serviceName,
				Annotations: map[string]string{
					externalDNSAnnotation: hostName,
				},
			}
			svc.Spec.Type = v1.ServiceTypeLoadBalancer
			svc.Spec.Selector = labels
			svc.Spec.Ports = []v1.ServicePort{
				{
					Port:       int32(port),
					TargetPort: intstr.FromInt(port),
				},
			}
		})
		framework.ExpectNoError(err, "failed to create service: %s in namespace: %s", serviceName, ns)

		By("Submitting the pod to kubernetes")
		route := fmt.Sprintf(`* -> inlineContent("%s") -> <shunt>`, "OK")
		pod := createSkipperPod(nameprefix, ns, route, labels, port)
		defer func() {
			By("deleting the pod")
			defer GinkgoRecover()
			err2 := cs.CoreV1().Pods(ns).Delete(ctx, pod.Name, metav1.DeleteOptions{})
			framework.ExpectNoError(err2, "failed to delete pod: %s in namespace: %s", pod.Name, ns)
		}()

		_, err = cs.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{})
		framework.ExpectNoError(err, "failed to create pod: %s in namespace: %s", pod.Name, ns)

		framework.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(ctx, f.ClientSet, pod.Name, pod.Namespace),
			"failed to wait for pod: %s in namespace: %s", pod.Name, ns)

		// wait for DNS and for pod to be reachable.
		By("Waiting up to " + timeout.String() + " for " + hostName + " to be reachable")
		err = waitForSuccessfulResponse(hostName, timeout)
		framework.ExpectNoError(err, "failed to wait for %s to be reachable", hostName)
	})

	f.It("Should create DNS entry via Service and AWS Load Balancer Controller [Zalando]", f.WithSlow(), func(ctx context.Context) {
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
			framework.ExpectNoError(err, "failed to delete service: %s in namespace: %s", serviceName, ns)
		}()

		hostName := fmt.Sprintf("%s-%d.%s", serviceName, time.Now().UTC().Unix(), E2EHostedZone())
		_, err := jigs.CreateLoadBalancerService(ctx, timeout, func(svc *v1.Service) {
			svc.ObjectMeta = metav1.ObjectMeta{
				Name: serviceName,
				Annotations: map[string]string{
					"service.beta.kubernetes.io/aws-load-balancer-type":            "external",
					"service.beta.kubernetes.io/aws-load-balancer-ip-address-type": "dualstack",
					"service.beta.kubernetes.io/aws-load-balancer-nlb-target-type": "ip",
					"service.beta.kubernetes.io/aws-load-balancer-scheme":          "internet-facing",
					externalDNSAnnotation: hostName,
				},
			}
			svc.Spec.Type = v1.ServiceTypeLoadBalancer
			svc.Spec.Selector = labels
			svc.Spec.Ports = []v1.ServicePort{
				{
					Port:       int32(port),
					TargetPort: intstr.FromInt(port),
				},
			}
		})
		framework.ExpectNoError(err, "failed to create service: %s in namespace: %s", serviceName, ns)

		By("Submitting the pod to kubernetes")
		route := fmt.Sprintf(`* -> inlineContent("%s") -> <shunt>`, "OK")
		pod := createSkipperPod(nameprefix, ns, route, labels, port)
		defer func() {
			By("deleting the pod")
			defer GinkgoRecover()
			err2 := cs.CoreV1().Pods(ns).Delete(ctx, pod.Name, metav1.DeleteOptions{})
			framework.ExpectNoError(err2, "failed to delete pod: %s in namespace: %s", pod.Name, ns)
		}()

		_, err = cs.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{})
		framework.ExpectNoError(err, "failed to create pod: %s in namespace: %s", pod.Name, ns)

		framework.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(ctx, f.ClientSet, pod.Name, pod.Namespace),
			"failed to wait for pod: %s in namespace: %s", pod.Name, ns)

		// wait for DNS and for pod to be reachable.
		By("Waiting up to " + timeout.String() + " for " + hostName + " to be reachable")
		err = waitForSuccessfulResponse(hostName, timeout)
		framework.ExpectNoError(err, "failed to wait for %s to be reachable", hostName)
	})

	f.It("Should create DNS entry via Ingress [Zalando]", f.WithSlow(), func(ctx context.Context) {
		nameprefix := serviceName + "-"
		ns := f.Namespace.Name
		labels := map[string]string{
			"foo": "bar",
			"baz": "blah",
		}
		port := 80

		By("Creating service " + serviceName + " in namespace " + ns)
		service := createServiceTypeClusterIP(serviceName, labels, port, port)
		defer func() {
			err := cs.CoreV1().Services(ns).Delete(ctx, serviceName, metav1.DeleteOptions{})
			framework.ExpectNoError(err, "failed to delete service: %s in namespace: %s", serviceName, ns)
		}()
		_, err := cs.CoreV1().Services(ns).Create(ctx, service, metav1.CreateOptions{})
		framework.ExpectNoError(err)

		hostName := fmt.Sprintf("%s-%d.%s", serviceName, time.Now().UTC().Unix(), E2EHostedZone())

		By("Creating an ingress with name " + serviceName + " in namespace " + ns + " with hostname " + hostName)
		ing := createIngress(serviceName, hostName, ns, "/", netv1.PathTypeImplementationSpecific, labels, nil, port)
		defer func() {
			err := cs.NetworkingV1().Ingresses(ns).Delete(ctx, ing.Name, metav1.DeleteOptions{})
			framework.ExpectNoError(err, "failed to delete ingress: %s in namespace: %s", serviceName, ns)
		}()
		ingressCreate, err := cs.NetworkingV1().Ingresses(ns).Create(ctx, ing, metav1.CreateOptions{})
		framework.ExpectNoError(err)

		_, err = jigi.WaitForIngressAddress(ctx, cs, ns, ingressCreate.Name, 10*time.Minute)
		framework.ExpectNoError(err)

		By("Submitting the pod to kubernetes")
		route := fmt.Sprintf(`* -> inlineContent("%s") -> <shunt>`, "OK")
		pod := createSkipperPod(nameprefix, ns, route, labels, port)
		defer func() {
			err := cs.CoreV1().Pods(ns).Delete(ctx, pod.Name, metav1.DeleteOptions{})
			framework.ExpectNoError(err, "failed to delete pod: %s in namespace: %s", pod.Name, ns)
		}()

		_, err = cs.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{})
		framework.ExpectNoError(err, "failed to create pod: %s in namespace: %s", pod.Name, ns)

		framework.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(ctx, f.ClientSet, pod.Name, pod.Namespace),
			"failed to wait for pod: %s in namespace: %s", pod.Name, ns)

		// wait for DNS and for pod to be reachable.
		By("Waiting up to " + timeout.String() + " for " + hostName + " to be reachable")
		err = waitForSuccessfulResponse(hostName, timeout)
		framework.ExpectNoError(err, "failed to wait for %s to be reachable", hostName)
	})
})
