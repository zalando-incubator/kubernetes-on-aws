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

	v1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"
)

func waitForServiceLoadBalancer(ctx context.Context, serviceName, ns string, cs kubernetes.Interface) (*v1.LoadBalancerIngress, error) {
	var lbIngress *v1.LoadBalancerIngress
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 10*time.Minute, true, func(_ context.Context) (done bool, err error) {
		svc, err := cs.CoreV1().Services(ns).Get(context.TODO(), serviceName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if len(svc.Status.LoadBalancer.Ingress) > 0 {
			lbIngress = &svc.Status.LoadBalancer.Ingress[0]
			return true, nil
		}
		return false, nil
	})
	return lbIngress, err
}

var _ = describe("Ingress canary test", func() {
	f := framework.NewDefaultFramework("skipper-ingress-canary-simple")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelBaseline
	var (
		cs kubernetes.Interface
	)

	It("Should create simple ingress canary with ServiceTypeLoadBalancer", func() {
		cs = f.ClientSet
		serviceName := "skipper-ingress-canary-test"
		ns := f.Namespace.Name
		hostName := fmt.Sprintf("%s-%d.%s", serviceName, time.Now().UTC().Unix(), E2EHostedZone())
		labels := map[string]string{
			"app": serviceName,
		}
		port := 8080
		targetPort := 9090
		response := "canary_test_ok"
		route := fmt.Sprintf(`* -> inlineContent("%s") -> <shunt>`, response)

		// Create a pod with skipper backend
		By("Creating a backend pod with " + serviceName + " in namespace " + ns)
		pod := createSkipperPod(serviceName, ns, route, labels, targetPort)
		_, err := cs.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})
		framework.ExpectNoError(err)
		framework.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(context.TODO(), f.ClientSet, pod.Name, pod.Namespace))

		//Create a service with type LoadBalancer
		loadBalancerServiceName := serviceName + "-lb"
		externalHostName := fmt.Sprintf("%s.%s", loadBalancerServiceName, E2EHostedZone())
		lbAnnotations := map[string]string{
			"external-dns.alpha.kubernetes.io/hostname": externalHostName,
		}
		By("Creating service with " + loadBalancerServiceName + " of ServiceTypeLoadBalancer in namespace " + ns)
		service := createServiceTypeLoadBalancer(loadBalancerServiceName, labels, lbAnnotations, port, targetPort)
		_, err = cs.CoreV1().Services(ns).Create(context.TODO(), service, metav1.CreateOptions{})
		framework.ExpectNoError(err)

		lb, err := waitForServiceLoadBalancer(context.TODO(), loadBalancerServiceName, ns, cs)
		framework.ExpectNoError(err)
		framework.Logf("Service LoadBalancer Ingress: %v", lb)

		//Create ClusterIP service for Ingress backend
		clusterIPServiceName := serviceName + "-clusterip"
		By("Creating service with " + clusterIPServiceName + " of ServiceTypeClusterIP type in namespace " + ns)
		clusterIPService := createServiceTypeClusterIP(clusterIPServiceName, labels, port, targetPort)
		_, err = cs.CoreV1().Services(ns).Create(context.TODO(), clusterIPService, metav1.CreateOptions{})
		framework.ExpectNoError(err)

		// Create an Ingress with the opt-out annotation
		ingAnnotations := map[string]string{
			"zalando.org/aws-load-balancer-type": "none",
		}
		By("Creating an ingress with " + clusterIPServiceName + " in namespace " + ns)
		ing := createIngress(serviceName, hostName, ns, "/", netv1.PathTypeImplementationSpecific, labels, ingAnnotations, port)
		_, err = cs.NetworkingV1().Ingresses(ns).Create(context.TODO(), ing, metav1.CreateOptions{})
		framework.ExpectNoError(err)

		_, err = cs.NetworkingV1().Ingresses(ns).Get(context.TODO(), ing.Name, metav1.GetOptions{ResourceVersion: "0"})
		framework.ExpectNoError(err)

		// TODO: Send requests to the LoadBalancer and Ingress endpoints and assert the responses are as expected.

	})
})
