package e2e

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/config"
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
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/api/networking/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
)

const (
	awsCliImage  = "registry.opensource.zalan.do/teapot/awscli:master-1"
	pauseImage   = "registry.opensource.zalan.do/teapot/pause-amd64:3.2"
	appLabelName = "application"
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

func createIngressV1(name, hostname, namespace, path string, pathType netv1.PathType, labels, annotations map[string]string, port int) *netv1.Ingress {
	return &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name + string(uuid.NewUUID()),
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: netv1.IngressSpec{
			Rules: []netv1.IngressRule{
				{
					Host: hostname,
					IngressRuleValue: netv1.IngressRuleValue{
						HTTP: &netv1.HTTPIngressRuleValue{
							Paths: []netv1.HTTPIngressPath{
								{
									PathType: &pathType,
									Path:     path,
									Backend: netv1.IngressBackend{
										Service: &netv1.IngressServiceBackend{
											Name: name,
											Port: netv1.ServiceBackendPort{
												Number: int32(port),
											},
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

func updateIngressV1(name, namespace, hostname, svcName, path string, pathType netv1.PathType, labels, annotations map[string]string, port int) *netv1.Ingress {
	return &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: netv1.IngressSpec{
			Rules: []netv1.IngressRule{
				{
					Host: hostname,
					IngressRuleValue: netv1.IngressRuleValue{
						HTTP: &netv1.HTTPIngressRuleValue{
							Paths: []netv1.HTTPIngressPath{
								{
									PathType: &pathType,
									Path:     path,
									Backend: netv1.IngressBackend{
										Service: &netv1.IngressServiceBackend{
											Name: svcName,
											Port: netv1.ServiceBackendPort{
												Number: int32(port),
											},
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

func addPathIngressV1(ing *netv1.Ingress, path string, pathType netv1.PathType, backend netv1.IngressBackend) *netv1.Ingress {
	addRules := []netv1.IngressRule{}
	origRules := ing.Spec.Rules

	for _, rule := range origRules {
		r := rule
		r.Host = rule.Host
		origPaths := r.IngressRuleValue.HTTP.Paths
		origPaths = append(origPaths, netv1.HTTPIngressPath{
			Path:     path,
			PathType: &pathType,
			Backend:  backend,
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
		Spec: createSkipperPodSpec(route, int32(port)),
	}
}

func createSkipperPodSpec(route string, port int32) corev1.PodSpec {
	return corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "skipper",
				Image: "registry.opensource.zalan.do/teapot/skipper:latest",
				Args: []string{
					"skipper",
					"-inline-routes",
					route,
					"-address",
					fmt.Sprintf(":%d", port),
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

func createAWSCLIPod(nameprefix, namespace string, args []string) *v1.Pod {
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
					Name:  "aws-cli",
					Image: awsCliImage,
					Args:  args,
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}
}

func createAWSIAMPod(nameprefix, namespace, s3Bucket string) *v1.Pod {
	pod := createAWSCLIPod(nameprefix, namespace, []string{"s3", "ls", fmt.Sprintf("s3://%s", s3Bucket)})
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
				Spec: createSkipperPodSpec(route, port),
			},
		},
	}
}

func pauseContainer() v1.Container {
	return v1.Container{
		Name:  "pause",
		Image: pauseImage,
		Resources: v1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("1m"),
				corev1.ResourceMemory: resource.MustParse("50Mi"),
			},
			Requests: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("1m"),
				corev1.ResourceMemory: resource.MustParse("50Mi"),
			},
		},
	}
}

// nodeTestPod returns a v1.Pod with the selector, tolerations and anti-affinity predicates that
// would result in the pod on a specific pool in a dedicated mode, with just one pod per node
func nodeTestPod(namespace string, poolName string, name string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels: map[string]string{
				"node-tests": "true",
			},
		},
		Spec: v1.PodSpec{
			Tolerations: []corev1.Toleration{
				{
					Key:      "dedicated",
					Operator: corev1.TolerationOpEqual,
					Value:    poolName,
					Effect:   corev1.TaintEffectNoSchedule,
				},
			},
			NodeSelector: map[string]string{
				"dedicated": poolName,
			},
			Affinity: &corev1.Affinity{
				PodAntiAffinity: &corev1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"dedicated": poolName,
								},
							},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			},
		},
	}
}

func nodeNameAffinity(nodeName string) *corev1.Affinity {
	return &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchFields: []corev1.NodeSelectorRequirement{
							{
								Key:      "metadata.name",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{nodeName},
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

func createImagePolicyWebhookTestDeployment(namePrefix, namespace, image, appLabel string, replicas int32) *appsv1.Deployment {
	zero := int64(0)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", namePrefix, uuid.NewUUID()),
			Namespace: namespace,
			Labels:    map[string]string{},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					appLabelName: appLabel,
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						appLabelName: appLabel,
					},
				},
				Spec: v1.PodSpec{
					TerminationGracePeriodSeconds: &zero,
					Containers: []v1.Container{
						{
							Name:  "image-policy-test",
							Image: image,
						},
					},
				},
			},
		},
	}
}

func createImagePolicyWebhookTestStatefulSet(namePrefix, namespace, image, appLabel string, replicas int32) *appsv1.StatefulSet {
	zero := int64(0)
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", namePrefix, uuid.NewUUID()),
			Namespace: namespace,
			Labels:    map[string]string{},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					appLabelName: appLabel,
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						appLabelName: appLabel,
					},
				},
				Spec: v1.PodSpec{
					TerminationGracePeriodSeconds: &zero,
					Containers: []v1.Container{
						{
							Name:  "image-policy-test",
							Image: image,
						},
					},
				},
			},
		},
	}
}

func createTestJob(namePrefix, name, namespace, image, appLabel string, args []string) *batchv1.Job {
	zero := int64(0)
	zero2 := int32(0)
	ten := int64(10)
	suspend := false
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", namePrefix, uuid.NewUUID()),
			Namespace: namespace,
			Labels:    map[string]string{},
		},
		Spec: batchv1.JobSpec{
			Suspend:               &suspend,
			ActiveDeadlineSeconds: &ten,
			BackoffLimit:          &zero2,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						appLabelName: appLabel,
					},
				},
				Spec: v1.PodSpec{
					RestartPolicy:                 v1.RestartPolicyOnFailure,
					TerminationGracePeriodSeconds: &zero,
					Containers: []v1.Container{
						{
							Name:  name,
							Image: image,
							Args:  args,
						},
					},
				},
			},
		},
	}
}

func createImagePolicyWebhookTestJob(namePrefix, namespace, image, appLabel string) *batchv1.Job {
	return createTestJob(namePrefix, "image-policy-test", namespace, image, appLabel, []string{})
}

func createImagePolicyWebhookTestPod(namePrefix, namespace, image, appLabel string) *v1.Pod {
	zero := int64(0)
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", namePrefix, string(uuid.NewUUID())),
			Namespace: namespace,
			Labels: map[string]string{
				appLabelName: appLabel,
			},
		},
		Spec: v1.PodSpec{
			TerminationGracePeriodSeconds: &zero,
			Containers: []v1.Container{
				{
					Name:  "image-policy-test",
					Image: image,
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

func appLabelSelector(value string) labels.Selector {
	return labels.SelectorFromSet(map[string]string{
		appLabelName: value,
	})
}

func describe(text string, body func()) bool {
	return Describe("[zalando] "+text, body)
}

// handleFlags sets up all flags and parses the command line.
func handleFlags() {
	config.CopyFlags(config.Flags, flag.CommandLine)
	framework.RegisterCommonFlags(flag.CommandLine)
	framework.RegisterClusterFlags(flag.CommandLine)
	flag.Parse()
}

func getenv(envar, def string) string {
	v := os.Getenv(envar)
	if v == "" {
		return def
	}
	return v
}

// E2EHostedZone returns the hosted zone defined for e2e test.
func E2EHostedZone() string {
	return getenv("HOSTED_ZONE", "example.org")
}

// E2EClusterAlias returns the alias of the cluster used for e2e tests.
func E2EClusterAlias() string {
	result, ok := os.LookupEnv("CLUSTER_ALIAS")
	if !ok {
		panic("CLUSTER_ALIAS not defined")
	}
	return result
}

// E2EClusterID returns the ID of the cluster used for e2e tests.
func E2EClusterID() string {
	result, ok := os.LookupEnv("CLUSTER_ID")
	if !ok {
		panic("CLUSTER_ID not defined")
	}
	return result
}

// E2ES3AWSIAMBucket returns the s3 bucket name used for AWS IAM e2e tests.
func E2ES3AWSIAMBucket() string {
	return getenv("S3_AWS_IAM_BUCKET", "")
}

// E2EAWSIAMRole returns the AWS IAM role used for AWS IAM e2e tests.
func E2EAWSIAMRole() string {
	return getenv("AWS_IAM_ROLE", "")
}
