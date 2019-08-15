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
	"errors"
	"fmt"
	"net"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = framework.KubeDescribe("Static Egress creation", func() {
	f := framework.NewDefaultFramework("egress")
	var cs kubernetes.Interface
	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("Should create valid static egress route [Egress] [Zalando]", func() {
		ips, err := net.LookupIP("readmyip.appspot.com")
		Expect(err).NotTo(HaveOccurred())

		knownCIDRs := []string{"216.58.192.0/19", "172.217.0.0/16"}
		nets := make([]*net.IPNet, 0, len(knownCIDRs))
		for _, cidr := range knownCIDRs {
			// ignore error since we know the input
			_, ipNet, _ := net.ParseCIDR(cidr)
			nets = append(nets, ipNet)
		}

		if !ipsInCIDRs(ips, nets) {
			Expect(fmt.Errorf("IPs %s of 'readmyip.appspot.com' are not in expected ranges: %s", ips, nets)).NotTo(HaveOccurred())
		}

		configmapName := "egress-test"
		ns := f.Namespace.Name

		labels := map[string]string{
			"egress": "static",
		}

		data := map[string]string{}

		for i, cidr := range knownCIDRs {
			data[fmt.Sprintf("readmyip.appspot.com-%d", i)] = cidr
		}

		// create Pod which finds out if it's public IP changes
		By("Creating a ping POD in namespace " + ns + " we are waiting for NAT gateway created and routes are populated")
		pingPod := createPingPod("ping", ns)
		defer func() {
			By("deleting the pod")
			defer GinkgoRecover()
			cs.CoreV1().Pods(ns).Delete(pingPod.Name, metav1.NewDeleteOptions(0))
			// don't care about POD deletion, because it should exit by itself
		}()
		_, err = cs.CoreV1().Pods(ns).Create(pingPod)
		Expect(err).NotTo(HaveOccurred())
		framework.ExpectNoError(f.WaitForPodRunning(pingPod.Name))

		// ConfigMap
		By("Creating configmap " + configmapName + " in namespace " + ns)
		cmap := createConfigMap(configmapName, ns, labels, data)
		defer func() {
			By("deleting the configmap")
			defer GinkgoRecover()
			err2 := cs.CoreV1().ConfigMaps(ns).Delete(cmap.Name, metav1.NewDeleteOptions(0))
			Expect(err2).NotTo(HaveOccurred())
		}()
		_, err = cs.CoreV1().ConfigMaps(ns).Create(cmap)
		Expect(err).NotTo(HaveOccurred())

		// wait for egress route and NAT GWs ready and POD exit code 0 vs 2
		Eventually(func() error {
			return containerExitStatus(cs, ns, pingPod.Name)
		}, 10 * time.Minute, 10 * time.Second).ShouldNot(HaveOccurred())
	})
})

func containerExitStatus(cs kubernetes.Interface, namespace string, pingPodName string) error {
	p, err := cs.CoreV1().Pods(namespace).Get(pingPodName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if p.Status.ContainerStatuses[0].State.Terminated == nil {
		return errors.New("container not terminated yet")
	}

	switch n := p.Status.ContainerStatuses[0].State.Terminated.ExitCode; n {
	case 0:
		return nil
	default:
		return fmt.Errorf("unexpected exit status: %d", n)
	}
}

func ipsInCIDRs(ips []net.IP, cidrs []*net.IPNet) bool {
	for _, ip := range ips {
		for _, cidr := range cidrs {
			if cidr.Contains(ip) {
				return true
			}
		}
	}
	return false
}
