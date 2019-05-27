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
	"fmt"
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
		configmapName := "egress-test"
		ns := f.Namespace.Name

		labels := map[string]string{
			"egress": "static",
		}

		data := map[string]string{
			"readmyip.appspot.com": "216.58.192.0/19",
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
		_, err := cs.CoreV1().Pods(ns).Create(pingPod)
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
		for {
			p, err := cs.CoreV1().Pods(ns).Get(pingPod.Name, metav1.GetOptions{})
			if err != nil {
				Expect(fmt.Errorf("Could not get POD %s", pingPod.Name)).NotTo(HaveOccurred())
				return
			}

			if p.Status.ContainerStatuses[0].State.Terminated == nil {
				time.Sleep(10 * time.Second)
				continue
			}

			switch n := p.Status.ContainerStatuses[0].State.Terminated.ExitCode; n {
			case 0:
				return
			case 2:
				// set error
				Expect(fmt.Errorf("failed to change public IP")).NotTo(HaveOccurred())
				return
			}
		}
	})
})
