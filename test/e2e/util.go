package e2e

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
)

func createIngress(name, hostname, namespace string, label map[string]string, port int) *v1beta1.Ingress {
	return &v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + string(uuid.NewUUID()),
			Namespace: namespace,
			Labels:    label,
		},
		Spec: v1beta1.IngressSpec{
			Backend: &v1beta1.IngressBackend{
				ServiceName: name,
				ServicePort: intstr.FromInt(port),
			},
			Rules: []v1beta1.IngressRule{
				{
					Host: hostname,
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Path: "/",
									Backend: v1beta1.IngressBackend{
										ServiceName: name,
										ServicePort: intstr.FromInt(port),
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

func createNginxDeployment(nameprefix, namespace string, label map[string]string, port, replicas int32) *appsv1.Deployment {
	zero := int64(0)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameprefix + string(uuid.NewUUID()),
			Namespace: namespace,
			Labels:    label,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: label},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: label,
				},
				Spec: v1.PodSpec{
					TerminationGracePeriodSeconds: &zero,
					Containers: []v1.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
							Ports: []v1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: port,
								},
							},
						},
					},
				},
			},
		},
	}
}

func createNginxPod(nameprefix, namespace string, labels map[string]string, port int) *v1.Pod {
	return &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameprefix + string(uuid.NewUUID()),
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
					Ports: []v1.ContainerPort{
						{
							Name:          "http",
							ContainerPort: int32(port),
						},
					},
				},
			},
		},
	}
}

func createPingPod(nameprefix, namespace string) *v1.Pod {
	return &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameprefix + string(uuid.NewUUID()),
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "check-change-myip",
					Image: "registry.opensource.zalan.do/teapot/check-change-myip:v0.0.1",
				},
			},
		},
	}
}

func createConfigMap(name, namespace string, labels, data map[string]string) *v1.ConfigMap {
	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Data: data,
	}
}

func createNginxDeploymentWithHostNetwork(nameprefix, namespace, serviceAccount string, label map[string]string, port, replicas int32) *appsv1.Deployment {
	zero := int64(0)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameprefix + string(uuid.NewUUID()),
			Namespace: namespace,
			Labels:    label,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: label},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: label,
				},
				Spec: v1.PodSpec{
					HostNetwork:                   true,
					ServiceAccountName:            serviceAccount,
					TerminationGracePeriodSeconds: &zero,
					Containers: []v1.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
							Ports: []v1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: port,
									HostPort:      port,
								},
							},
						},
					},
				},
			},
		},
	}
}

func createServiceAccount(namespace, serviceAccount string) *v1.ServiceAccount {
	trueValue := true
	return &v1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccount,
			Namespace: namespace,
		},
		AutomountServiceAccountToken: &trueValue,
	}

}

func createNginxPodWithHostNetwork(namespace, serviceAccount string, label map[string]string, port int32) *v1.Pod {
	return &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "psp-test-" + string(uuid.NewUUID()),
			Namespace: namespace,
			Labels:    label,
		},
		Spec: v1.PodSpec{
			HostNetwork:        true,
			ServiceAccountName: serviceAccount,
			Containers: []v1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
					Ports: []v1.ContainerPort{
						{
							Name:          "http",
							ContainerPort: port,
							HostPort:      port,
						},
					},
				},
			},
		},
	}
}

func createServiceTypeClusterIP(serviceName string, labels map[string]string, port, targetPort int) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   serviceName,
			Labels: labels,
		},
		Spec: v1.ServiceSpec{
			Type:     v1.ServiceTypeClusterIP,
			Selector: labels,
			Ports: []v1.ServicePort{{
				Port:       int32(port),
				TargetPort: intstr.FromInt(targetPort),
			}},
		},
	}
}

func createServiceTypeLoadbalancer(serviceName, hostName string, labels map[string]string, port int) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
			Annotations: map[string]string{
				externalDNSAnnotation: hostName,
			},
		},
		Spec: v1.ServiceSpec{
			Type:     v1.ServiceTypeLoadBalancer,
			Selector: labels,
			Ports: []v1.ServicePort{{
				Port:       int32(port),
				TargetPort: intstr.FromInt(port),
			}},
		},
	}
}

func waitForSuccessfulResponse(hostname string, timeout time.Duration) error {
	client := http.Client{
		Transport: &http.Transport{},
		Timeout:   10 * time.Second,
	}

	url, err := url.Parse(hostname)
	if err != nil {
		return err
	}

	url.Scheme = "http"

	host := url.String()

	req, err := http.NewRequest("GET", host, nil)
	if err != nil {
		return err
	}

	timeoutEnd := time.Now().UTC().Add(timeout)

	for time.Now().UTC().Before(timeoutEnd) {
		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(10 * time.Second)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return nil
		}
	}

	return fmt.Errorf("%s was not reachable after %s", host, timeout)
}

func isRedirect(code int) bool {
	return code >= 300 && code <= 399
}

func isSuccess(code int) bool {
	return code == 200
}

func waitForResponse(hostname, scheme string, timeout time.Duration, expectedCode func(int) bool, insecure bool) error {
	localTimeout := 10 * time.Second
	if timeout < localTimeout {
		localTimeout = timeout
	}
	url, err := url.Parse(hostname)
	if err != nil {
		return err
	}

	url.Scheme = scheme

	host := url.String()

	req, err := http.NewRequest("GET", host, nil)
	if err != nil {
		return err
	}

	timeoutEnd := time.Now().UTC().Add(timeout)

	for time.Now().UTC().Before(timeoutEnd) {
		t := &http.Transport{}
		if insecure {
			t = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
		}
		client := http.Client{
			Transport: t,
			Timeout:   localTimeout,
			CheckRedirect: func(r *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(localTimeout)
			continue
		}
		defer resp.Body.Close()

		if expectedCode(resp.StatusCode) {
			return nil
		}
	}

	return fmt.Errorf("%s was not reachable after %s", host, timeout)
}

func waitForReplicas(deploymentName, namespace string, kubeClient kubernetes.Interface, timeout time.Duration, desiredReplicas int) {
	interval := 20 * time.Second
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		deployment, err := kubeClient.AppsV1().Deployments(namespace).Get(deploymentName, metav1.GetOptions{})
		if err != nil {
			framework.Failf("Failed to get replication controller %s: %v", deployment, err)
		}
		replicas := int(deployment.Status.ReadyReplicas)
		framework.Logf("waiting for %d replicas (current: %d)", desiredReplicas, replicas)
		return replicas == desiredReplicas, nil // Expected number of replicas found. Exit.
	})
	if err != nil {
		framework.Failf("Timeout waiting %v for %v replicas", timeout, desiredReplicas)
	}
}

/** needed for image webhook policy tests: */

func createImagePolicyWebhookTestDeployment(nameprefix, namespace, tag, podname string, replicas int32) *v1beta1.Deployment {
	zero := int64(0)
	return &v1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameprefix + string(uuid.NewUUID()),
			Namespace: namespace,
			Labels:    map[string]string{},
		},
		Spec: v1beta1.DeploymentSpec{
			Replicas: &replicas,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": podname,
					},
				},
				Spec: v1.PodSpec{
					TerminationGracePeriodSeconds: &zero,
					Containers: []v1.Container{
						{
							Name:  "image-policy-webhook-test",
							Image: fmt.Sprintf("registry.opensource.zalan.do/teapot/image-policy-webhook-test:%s", tag),
						},
					},
				},
			},
		},
	}
}

func createVegetaDeployment(hostPath string, rate int) *appsv1.Deployment {
	replicas := int32(1)
	cmd := fmt.Sprintf("echo 'GET https://%s' | vegeta attack -rate=%d", hostPath, rate)

	name := "example-app-vegeta"
	labels := map[string]string{
		"application": name,
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    name,
							Image:   "peterevans/vegeta",
							Command: []string{"sh", "-c"},
							Args:    []string{cmd},
							Resources: corev1.ResourceRequirements{
								Limits: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceMemory: resource.MustParse("100Mi"),
								},
								Requests: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("100Mi"),
								},
							},
						},
					},
				},
			},
		},
	}
}
