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
	"regexp"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	. "github.com/onsi/ginkgo/v2"
)

var _ = describe("GPU job processing", func() {
	f := framework.NewDefaultFramework("gpu")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelBaseline
	var cs kubernetes.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	f.It("Should run a vector-add job on a gpu node [Zalando] [GPU]", f.WithSlow(), func(ctx context.Context) {
		runGPUTest(ctx, f, cs, "gpu-test-", "nvcr.io/nvidia/k8s/cuda-sample:vectoradd-cuda12.5.0-ubi8", nil, "PASSED")
	})

	f.It("Should compile and run a CUDA kernel on a gpu node [Zalando] [GPU]", f.WithSlow(), func(ctx context.Context) {
		runGPUTest(ctx, f, cs, "gpu-test-", "nvidia/cuda:13.2.1-devel-ubuntu24.04", []string{"bash", "-c", `cat > /tmp/t.cu <<EOF
#include <cstdio>
__global__ void add(int *a){ *a += 41; }
int main(){int *d,h=1; cudaMalloc(&d,4); cudaMemcpy(d,&h,4,cudaMemcpyHostToDevice);add<<<1,1>>>(d); cudaDeviceSynchronize();cudaError_t e=cudaGetLastError();if(e){ printf("FAIL: %s\n", cudaGetErrorString(e)); return 1; }cudaMemcpy(&h,d,4,cudaMemcpyDeviceToHost);printf("%s (result=%d)\n", h==42?"PASSED":"FAILED", h); return h==42?0:1;}
EOF
nvcc --version | grep release
nvcc -o /tmp/t /tmp/t.cu && /tmp/t`}, "PASSED")
	})

	f.It("Should run a PyTorch CUDA job on a gpu node [Zalando] [GPU]", f.WithSlow(), func(ctx context.Context) {
		runGPUTest(ctx, f, cs, "gpu-test-", "pytorch/pytorch:2.12.1-cuda13.2-cudnn9-runtime", []string{"python", "-c",
			"import torch; v=torch.version.cuda; assert torch.cuda.is_available(); " +
				"assert tuple(map(int,v.split('.')))>=(13,2), f'CUDA {v} < 13.2'; " +
				"x=torch.rand(3,device='cuda'); torch.cuda.synchronize(); " +
				"print(f'PASSED: torch {torch.__version__}, CUDA {v}, {torch.cuda.get_device_name()}')",
		}, "PASSED")
	})
})

func runGPUTest(ctx context.Context, f *framework.Framework, cs kubernetes.Interface, nameprefix, image string, command []string, logPattern string) {
	ns := f.Namespace.Name
	labels := map[string]string{
		"application": "vector-add",
	}

	By("Creating a vector pod which runs on a GPU node")
	pod := createVectorPod(nameprefix, ns, labels, image, command)
	_, err := cs.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{})
	framework.ExpectNoError(err, "Could not create Pod %s", pod.Name)
	framework.ExpectNoError(e2epod.WaitForPodSuccessInNamespaceTimeout(ctx, f.ClientSet, pod.Name, pod.Namespace, 15*time.Minute))
	deadline := time.Now().Add(2 * time.Minute)
	for {
		if time.Now().After(deadline) {
			framework.Failf("Pod %s did not reach Terminated state within timeout", pod.Name)
		}

		p, err := cs.CoreV1().Pods(ns).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			framework.ExpectNoError(err, "Could not get Pod %s", pod.Name)
			return
		}
		if p.Status.ContainerStatuses[0].State.Terminated == nil {
			time.Sleep(10 * time.Second)
			continue
		}
		n := p.Status.ContainerStatuses[0].State.Terminated.ExitCode
		if n != 0 {
			framework.ExpectNoError(fmt.Errorf("expected Pod %s to terminate with exit code 0", pod.Name))
			return
		}
		logs, err := getPodLogs(cs, ns, pod.Name, "main", false)
		framework.ExpectNoError(err, "Should be able to get logs for pod %v", pod.Name)
		regex := regexp.MustCompile(logPattern)
		if !regex.MatchString(logs) {
			framework.Failf("Expected log pattern %q not found in logs of pod %s:\n%s", logPattern, pod.Name, logs)
		}
		return
	}
}
