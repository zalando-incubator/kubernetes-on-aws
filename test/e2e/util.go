package e2e

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	testutil "k8s.io/kubernetes/test/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rgclient "github.com/szuecs/routegroup-client"
	rgv1 "github.com/szuecs/routegroup-client/apis/zalando.org/v1"
	zv1 "github.com/zalando-incubator/kube-aws-iam-controller/pkg/apis/zalando.org/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/networking/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
)

var (
	errTimeout      = errors.New("Timeout")
	poll            = 2 * time.Second
	pollLongTimeout = 5 * time.Minute
)

// type ConditionFunc func() (done bool, err error)
// Poll(interval, timeout time.Duration, condition ConditionFunc)
func waitForRouteGroup(cs rgclient.ZalandoInterface, name, ns string, d time.Duration) (string, error) {
	var addr string
	err := wait.Poll(10*time.Second, d, func() (done bool, err error) {
		rg, err := cs.ZalandoV1().RouteGroups(ns).Get(context.TODO(), name, metav1.GetOptions{ResourceVersion: "0"})
		if err != nil {
			return true, err
		}
		if len(rg.Status.LoadBalancer.RouteGroup) > 0 {
			addr = rg.Status.LoadBalancer.RouteGroup[0].Hostname
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return "", fmt.Errorf("Failed to get active load balancer for Routegroup %s/%s: %w", name, ns, err)
	}

	return addr, err
}

func createRouteGroup(name, hostname, namespace string, labels, annotations map[string]string, port int, routes ...rgv1.RouteGroupRouteSpec) *rgv1.RouteGroup {
	return &rgv1.RouteGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name + string(uuid.NewUUID()),
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: rgv1.RouteGroupSpec{
			Hosts: []string{hostname},
			Backends: []rgv1.RouteGroupBackend{
				{
					Name:        name,
					Type:        "service",
					ServiceName: name,
					ServicePort: port,
				},
				{
					Name: "router",
					Type: "shunt",
				},
			},
			DefaultBackends: []rgv1.RouteGroupBackendReference{
				{
					BackendName: name,
					Weight:      1,
				},
			},
			Routes: routes,
		},
	}
}

func createRouteGroupWithBackends(name, hostname, namespace string, labels, annotations map[string]string, port int, backends []rgv1.RouteGroupBackend, routes ...rgv1.RouteGroupRouteSpec) *rgv1.RouteGroup {
	return &rgv1.RouteGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name + string(uuid.NewUUID()),
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: rgv1.RouteGroupSpec{
			Hosts:    []string{hostname},
			Backends: backends,
			Routes:   routes,
		},
	}
}

func createIngress(name, hostname, namespace string, labels, annotations map[string]string, port int) *v1beta1.Ingress {
	return &v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name + string(uuid.NewUUID()),
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: v1beta1.IngressSpec{
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

func updateIngress(name, namespace, hostname, svcName, path string, labels, annotations map[string]string, port int) *v1beta1.Ingress {
	return &v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: hostname,
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Path: path,
									Backend: v1beta1.IngressBackend{
										ServiceName: svcName,
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

func addHostIngress(ing *v1beta1.Ingress, hostnames ...string) *v1beta1.Ingress {
	addRules := []v1beta1.IngressRule{}
	origRules := ing.Spec.Rules

	for _, hostname := range hostnames {
		for _, rule := range origRules {
			r := rule
			r.Host = hostname
			addRules = append(addRules, r)
		}
	}
	ing.Spec.Rules = append(origRules, addRules...)
	return ing
}

func addPathIngress(ing *v1beta1.Ingress, path string, backend v1beta1.IngressBackend) *v1beta1.Ingress {
	addRules := []v1beta1.IngressRule{}
	origRules := ing.Spec.Rules

	for _, rule := range origRules {
		r := rule
		r.Host = rule.Host
		origPaths := r.IngressRuleValue.HTTP.Paths
		origPaths = append(origPaths, v1beta1.HTTPIngressPath{
			Path:    path,
			Backend: backend,
		})
		r.IngressRuleValue.HTTP.Paths = origPaths
		addRules = append(addRules, r)
	}
	ing.Spec.Rules = addRules
	return ing
}

func changePathIngress(ing *v1beta1.Ingress, path string) *v1beta1.Ingress {
	return updateIngress(
		ing.ObjectMeta.Name,
		ing.ObjectMeta.Namespace,
		ing.Spec.Rules[0].Host,
		ing.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServiceName,
		path,
		ing.ObjectMeta.Labels,
		ing.ObjectMeta.Annotations,
		ing.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntValue(),
	)
}

func createSkipperPodWithHostNetwork(nameprefix, namespace, serviceAccount, route string, labels map[string]string, port int) *v1.Pod {
	pod := createSkipperPod(nameprefix, namespace, route, labels, port)
	pod.Spec.HostNetwork = true
	pod.Spec.ServiceAccountName = serviceAccount
	pod.Spec.Containers[0].Ports[0].HostPort = int32(port)
	return pod
}

func createSkipperPod(nameprefix, namespace, route string, labels map[string]string, port int) *v1.Pod {
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
					Name:  "skipper",
					Image: "registry.opensource.zalan.do/pathfinder/skipper:v0.11.107",
					Args: []string{
						"skipper",
						"-inline-routes",
						route,
						fmt.Sprintf("-address=:%d", port),
					},
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
					Image: "registry.opensource.zalan.do/teapot/check-change-myip:master-2",
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
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

func createAWSCLIPod(nameprefix, namespace, s3Bucket string) *v1.Pod {
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
					Name:    "aws-cli",
					Image:   "alpine:3.9",
					Command: []string{"/bin/sh", "-c"},
					Args: []string{
						fmt.Sprintf(`
apk add -U py-pip;
pip install awscli;
aws s3 ls s3://%s`, s3Bucket),
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}
}

func createAWSIAMPod(nameprefix, namespace, s3Bucket string) *v1.Pod {
	pod := createAWSCLIPod(nameprefix, namespace, s3Bucket)
	pod.Spec.Containers[0].Env = []v1.EnvVar{
		{
			Name:  "AWS_SHARED_CREDENTIALS_FILE",
			Value: "/meta/aws-iam/credentials.process",
		},
	}
	pod.Spec.Containers[0].VolumeMounts = []v1.VolumeMount{
		{
			Name:      "aws-iam-credentials",
			MountPath: "/meta/aws-iam",
			ReadOnly:  true,
		},
	}
	pod.Spec.Volumes = []v1.Volume{
		{
			Name: "aws-iam-credentials",
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: "aws-iam-test",
				},
			},
		},
	}
	return pod
}

func createAWSIAMRole(name, namespace, role string) *zv1.AWSIAMRole {
	return &zv1.AWSIAMRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: zv1.AWSIAMRoleSpec{
			RoleReference:       role,
			RoleSessionDuration: 3600,
		},
	}
}

func createSkipperBackendDeploymentWithHostNetwork(nameprefix, namespace, serviceAccount, route string, label map[string]string, port, replicas int32) *appsv1.Deployment {
	depl := createSkipperBackendDeployment(nameprefix, namespace, route, label, port, replicas)
	depl.Spec.Template.Spec.ServiceAccountName = serviceAccount
	return depl
}

func createSkipperBackendDeployment(nameprefix, namespace, route string, label map[string]string, port, replicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameprefix + string(uuid.NewUUID()),
			Namespace: namespace,
			Labels:    label,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: label},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: label,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "skipper",
							Image: "registry.opensource.zalan.do/pathfinder/skipper:v0.11.35",
							Args: []string{
								"skipper",
								"-inline-routes",
								route,
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: port,
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("250Mi"),
								},
								Requests: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("250Mi"),
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

func createRBACRoleBindingSA(role, namespace, serviceAccount string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccount,
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     role,
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
	return code == http.StatusOK
}

func isNotFound(code int) bool {
	return code == http.StatusNotFound
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

func waitForResponseReturnResponse(req *http.Request, timeout time.Duration, expectedCode func(int) bool, insecure bool) (*http.Response, error) {
	localTimeout := 10 * time.Second
	if timeout < localTimeout {
		localTimeout = timeout
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
			e2elog.Logf("%s localtimeout", req.URL.String())
			time.Sleep(localTimeout)
			continue
		}
		//e2elog.Logf("%s , header Foo: '%s', status code: %d", req.URL.String(), req.Header.Get("Foo"), resp.StatusCode)
		if expectedCode(resp.StatusCode) {
			return resp, nil
		}
		resp.Body.Close()
		time.Sleep(time.Second)
		client.CloseIdleConnections()
	}

	return nil, fmt.Errorf("%s was not reachable after %s", req.URL.String(), timeout)
}

func waitForReplicas(deploymentName, namespace string, kubeClient kubernetes.Interface, timeout time.Duration, desiredReplicas int) {
	interval := 20 * time.Second
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		deployment, err := kubeClient.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
		if err != nil {
			framework.Failf("Failed to get replication controller %s: %v", deployment, err)
		}
		replicas := int(deployment.Status.ReadyReplicas)
		e2elog.Logf("waiting for %d replicas (current: %d)", desiredReplicas, replicas)
		return replicas == desiredReplicas, nil // Expected number of replicas found. Exit.
	})
	if err != nil {
		framework.Failf("Timeout waiting %v for %v replicas", timeout, desiredReplicas)
	}
}

/** needed for image webhook policy tests: */

func createImagePolicyWebhookTestDeployment(nameprefix, namespace, tag, podname string, replicas int32) *appsv1.Deployment {
	zero := int64(0)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameprefix + string(uuid.NewUUID()),
			Namespace: namespace,
			Labels:    map[string]string{},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": podname,
				},
			},
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

const NVIDIAGPUResourceName corev1.ResourceName = "nvidia.com/gpu"

func createVectorPod(nameprefix, namespace string, labels map[string]string) *v1.Pod {
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
					Name:  "cuda-vector-add",
					Image: "k8s.gcr.io/cuda-vector-add:v0.1",
					Resources: corev1.ResourceRequirements{
						Limits: v1.ResourceList{
							NVIDIAGPUResourceName: *resource.NewQuantity(1, resource.DecimalSI),
						},
					},
				},
			},
		},
	}
}
func deleteDeployment(cs kubernetes.Interface, ns string, deployment *appsv1.Deployment) {
	By(fmt.Sprintf("Delete a compliant deployment: %s", deployment.Name))
	defer GinkgoRecover()
	err := cs.AppsV1().Deployments(ns).Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{})
	Expect(err).NotTo(HaveOccurred())
}

func createHTTPRoundTripper() (http.RoundTripper, chan<- struct{}) {
	tr := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		TLSHandshakeTimeout: 5 * time.Second,
		IdleConnTimeout:     5 * time.Second,
	}
	ch := make(chan struct{})
	go func(transport *http.Transport, quit <-chan struct{}) {
		for {
			select {
			case <-time.After(3 * time.Second):
				transport.CloseIdleConnections()
			case <-quit:
				return
			}
		}
	}(tr, ch)
	return tr, ch
}

func getAndWaitResponse(rt http.RoundTripper, req *http.Request, timeout time.Duration, expectedStatusCode int) (resp *http.Response, err error) {
	d := 1 * time.Second
	if timeout < d {
		d = timeout - 1
	}
	timeoutCH := make(chan struct{})
	go func() {
		time.Sleep(timeout)
		timeoutCH <- struct{}{}
	}()

	for {
		resp, err = rt.RoundTrip(req)
		if err == nil && resp.StatusCode == expectedStatusCode {
			return
		}
		if err != nil {
			log.Printf("Failed to do rountrip: %v", err)
		}

		select {
		case <-timeoutCH:
			log.Printf("timeout to GET %s", req.URL)
			return
		case <-time.After(d):
			log.Printf("retry to GET %s", req.URL)
			continue
		}
	}
}

func getBody(resp *http.Response) (string, error) {
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("response code from backend: %d", resp.StatusCode)
	}
	b := make([]byte, 0, 1024)
	buf := bytes.NewBuffer(b)
	if _, err := io.Copy(buf, resp.Body); err != nil {
		return "", fmt.Errorf("failed to copy body: %v", err)
	}
	return buf.String(), nil
}

func getPodLogs(c kubernetes.Interface, namespace, podName, containerName string, previous bool) (string, error) {
	logs, err := c.CoreV1().RESTClient().Get().
		Resource("pods").
		Namespace(namespace).
		Name(podName).SubResource("log").
		Param("container", containerName).
		Param("previous", strconv.FormatBool(previous)).
		Do(context.TODO()).
		Raw()
	if err != nil {
		return "", err
	}
	if err == nil && strings.Contains(string(logs), "Internal Error") {
		return "", fmt.Errorf("Fetched log contains \"Internal Error\": %q", string(logs))
	}
	return string(logs), err
}

// waitForDeploymentWithCondition waits for the specified deployment condition.
func waitForDeploymentWithCondition(c clientset.Interface, ns, deploymentName, reason string, condType appsv1.DeploymentConditionType) error {
	return testutil.WaitForDeploymentWithCondition(c, ns, deploymentName, reason, condType, framework.Logf, poll, pollLongTimeout)
}
