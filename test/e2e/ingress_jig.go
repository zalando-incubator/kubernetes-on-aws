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

// Local replacement for k8s.io/kubernetes/test/e2e/framework/ingress which was
// removed in Kubernetes v1.36.
package e2e

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// ingressTestJig is a minimal replacement for the removed upstream ingress.TestJig.
type ingressTestJig struct {
	Client kubernetes.Interface
}

// newIngressTestJig creates a new ingressTestJig.
func newIngressTestJig(c kubernetes.Interface) *ingressTestJig {
	return &ingressTestJig{Client: c}
}

// WaitForIngressAddress polls until the Ingress has an address in its load
// balancer status, then returns it.
func (j *ingressTestJig) WaitForIngressAddress(ctx context.Context, c kubernetes.Interface, ns, ingName string, timeout time.Duration) (string, error) {
	var address string
	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		ing, err := c.NetworkingV1().Ingresses(ns).Get(ctx, ingName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		for _, a := range ing.Status.LoadBalancer.Ingress {
			if a.Hostname != "" {
				address = a.Hostname
				return true, nil
			}
			if a.IP != "" {
				address = a.IP
				return true, nil
			}
		}
		return false, nil
	})
	return address, err
}
