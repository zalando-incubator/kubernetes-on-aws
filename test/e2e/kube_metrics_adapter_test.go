package e2e

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscaling "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/ingress"
)

// Test Scale down with custom metrics from an app's /metrics endpoint
// 1. Create a deployment with an hpa and more pods than needed. Then "deploy" it
// 2. Set the metrics "queue_count" of the app at startup
// 3. Observe if the HPA downscales
var _ = framework.KubeDescribe("[HPA] Horizontal pod autoscaling (scale resource: Custom Metrics from kube-metrics-adapter)", func() {
	f := framework.NewDefaultFramework("zalando-kube-metrics-adapter")
	var cs kubernetes.Interface
	var jig *ingress.TestJig

	const (
		DeploymentName = "sample-custom-metrics-autoscaling-e2e"
	)

	BeforeEach(func() {
		jig = ingress.NewIngressTestJig(f.ClientSet)
		cs = f.ClientSet
	})

	It("should scale down with Custom Metric of type Pod from kube-metrics-adapter [CustomMetricsAutoscaling] [Zalando]", func() {
		initialReplicas := 2
		scaledReplicas := 1
		metricValue := int64(10)
		metricName := "queue-count"
		metricTarget := metricValue * 2

		tc := CustomMetricTestCase{
			framework:       f,
			kubeClient:      cs,
			initialReplicas: initialReplicas,
			scaledReplicas:  scaledReplicas,
			deployment:      simplePodMetricDeployment(DeploymentName, int32(initialReplicas), metricName, metricValue),
			hpa:             simplePodMetricHPA(DeploymentName, metricName, metricTarget),
		}
		tc.Run()

	})

	It("should scale down with Custom Metric of type Object from Skipper [Ingress] [CustomMetricsAutoscaling] [Zalando]", func() {
		hostName := fmt.Sprintf("%s-%d.%s", DeploymentName, time.Now().UTC().Unix(), E2EHostedZone())

		initialReplicas := 2
		scaledReplicas := 1
		metricValue := 10
		metricTarget := int64(metricValue) * 2
		labels := map[string]string{
			"application": DeploymentName,
		}
		port := 80
		targetPort := 8000
		targetUrl := hostName + "/metrics"
		ingress := createIngress(DeploymentName, hostName, f.Namespace.Name, labels, nil, port)
		tc := CustomMetricTestCase{
			framework:       f,
			kubeClient:      cs,
			jig:             jig,
			initialReplicas: initialReplicas,
			scaledReplicas:  scaledReplicas,
			deployment:      simplePodDeployment(DeploymentName, int32(initialReplicas)),
			ingress:         ingress,
			hpa:             rpsBasedHPA(DeploymentName, ingress.Name, "extensions/v1beta1", metricTarget),
			service:         createServiceTypeClusterIP(DeploymentName, labels, 80, targetPort),
			auxDeployments: []*appsv1.Deployment{
				createVegetaDeployment(targetUrl, metricValue),
			},
		}
		tc.Run()
	})

	// TODO: this is almost identical to the test above, but tests that the
	// HPA can scale when the referenced ingress uses the networking.k8s.io
	// apiGroup
	It("should scale down with Custom Metric of type Object from Skipper (networking.k8s.io) [Ingress] [CustomMetricsAutoscaling] [Zalando]", func() {
		hostName := fmt.Sprintf("%s-%d.%s", DeploymentName, time.Now().UTC().Unix(), E2EHostedZone())

		initialReplicas := 2
		scaledReplicas := 1
		metricValue := 10
		metricTarget := int64(metricValue) * 2
		labels := map[string]string{
			"application": DeploymentName,
		}
		port := 80
		targetPort := 8000
		targetUrl := hostName + "/metrics"
		ingress := createIngress(DeploymentName, hostName, f.Namespace.Name, labels, nil, port)
		tc := CustomMetricTestCase{
			framework:       f,
			kubeClient:      cs,
			jig:             jig,
			initialReplicas: initialReplicas,
			scaledReplicas:  scaledReplicas,
			deployment:      simplePodDeployment(DeploymentName, int32(initialReplicas)),
			ingress:         ingress,
			hpa:             rpsBasedHPA(DeploymentName, ingress.Name, "networking.k8s.io/v1beta1", metricTarget),
			service:         createServiceTypeClusterIP(DeploymentName, labels, 80, targetPort),
			auxDeployments: []*appsv1.Deployment{
				createVegetaDeployment(targetUrl, metricValue),
			},
		}
		tc.Run()
	})
})

type CustomMetricTestCase struct {
	framework       *framework.Framework
	hpa             *autoscaling.HorizontalPodAutoscaler
	kubeClient      kubernetes.Interface
	jig             *ingress.TestJig
	deployment      *appsv1.Deployment
	pod             *corev1.Pod
	initialReplicas int
	scaledReplicas  int
	ingress         *v1beta1.Ingress
	service         *corev1.Service
	auxDeployments  []*appsv1.Deployment
}

func (tc *CustomMetricTestCase) Run() {
	By("By creating a deployment with an HPA and custom metrics Configured")
	ns := tc.framework.Namespace.Name

	// Create a MetricsExporter deployment
	_, err := tc.kubeClient.AppsV1().Deployments(ns).Create(context.TODO(), tc.deployment, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
	// Wait for the deployment to run
	waitForReplicas(tc.deployment.ObjectMeta.Name, tc.framework.Namespace.ObjectMeta.Name, tc.kubeClient, 15*time.Minute, tc.initialReplicas)

	for _, deployment := range tc.auxDeployments {
		_, err := tc.kubeClient.AppsV1().Deployments(ns).Create(context.TODO(), deployment, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		// Wait for the deployment to run
		waitForReplicas(deployment.ObjectMeta.Name, tc.framework.Namespace.ObjectMeta.Name, tc.kubeClient, 15*time.Minute, int(*(deployment.Spec.Replicas)))

	}

	// Check if an Ingress needs to be created
	if tc.ingress != nil {
		// Create a Service for the Ingress
		_, err = tc.kubeClient.CoreV1().Services(ns).Create(context.TODO(), tc.service, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		// Create an Ingress since RPS based scaling relies on it
		ingressCreate, err := tc.kubeClient.NetworkingV1beta1().Ingresses(ns).Create(context.TODO(), tc.ingress, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		_, err = tc.jig.WaitForIngressAddress(tc.kubeClient, ns, ingressCreate.Name, 10*time.Minute)
		Expect(err).NotTo(HaveOccurred())

	}
	// Autoscale the deployment
	_, err = tc.kubeClient.AutoscalingV2beta1().HorizontalPodAutoscalers(ns).Create(context.TODO(), tc.hpa, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	waitForReplicas(tc.deployment.ObjectMeta.Name, tc.framework.Namespace.ObjectMeta.Name, tc.kubeClient, 15*time.Minute, tc.scaledReplicas)
}

func cleanDeploymentToScale(f *framework.Framework, kubeClient kubernetes.Interface, deployment *appsv1.Deployment) {
	if deployment != nil {
		// Can't do much if there's an error while deleting the deployment, or can we?
		_ = kubeClient.AppsV1().Deployments(f.Namespace.Name).Delete(context.TODO(), deployment.ObjectMeta.Name, metav1.DeleteOptions{})
	}
}

// CustomMetricContainerSpec allows to specify a config for simplePodMetricDeployment
// with multiple containers exporting different metrics.
type CustomMetricContainerSpec struct {
	Name        string
	MetricName  string
	MetricValue int64
}

// simplePodMetricDeployment is a Deployment of simple application that exports a metric of
// fixed value to kube-metrics-adapter in a loop.
func simplePodMetricDeployment(name string, replicas int32, metricName string, metricValue int64) *appsv1.Deployment {
	return podMetricDeployment(name, replicas,
		[]CustomMetricContainerSpec{
			{
				Name:        "metrics-exporter-e2e",
				MetricName:  metricName,
				MetricValue: metricValue,
			},
		})
}

// podDeployment is a Deployment of an application that exposes an HTTP endpoint
func simplePodDeployment(name string, replicas int32) *appsv1.Deployment {
	podSpec := corev1.PodSpec{Containers: []corev1.Container{}}
	podSpec.Containers = append(podSpec.Containers, podContainerSpec(name))

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"application": name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"application": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"application": name,
					},
				},
				Spec: podSpec,
			},
			Replicas: &replicas,
		},
	}
}

// podMetricDeployment is a Deployment of an application that can expose
// an arbitrary amount of metrics of fixed value to kube-metrics-adapter in a loop. Each metric
// is exposed by a different container in one pod.
// The metric names and values are configured via the containers parameter.
func podMetricDeployment(name string, replicas int32, containers []CustomMetricContainerSpec) *appsv1.Deployment {
	podSpec := corev1.PodSpec{Containers: []corev1.Container{}}
	for _, containerSpec := range containers {
		podSpec.Containers = append(podSpec.Containers, podMetricContainerSpec(containerSpec))
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"application": name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"application": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"application": name,
					},
				},
				Spec: podSpec,
			},
			Replicas: &replicas,
		},
	}
}

func podContainerSpec(name string) corev1.Container {
	return corev1.Container{
		Name:  name,
		Image: "pierone.stups.zalan.do/teapot/sample-custom-metrics-autoscaling:master-13",
		Ports: []corev1.ContainerPort{{ContainerPort: 8000, Protocol: "TCP"}},
		Resources: corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse("300Mi"),
			},
			Requests: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("300Mi"),
			},
		},
	}
}

func podMetricContainerSpec(container CustomMetricContainerSpec) corev1.Container {
	return corev1.Container{
		Name:  container.Name,
		Image: "pierone.stups.zalan.do/teapot/sample-custom-metrics-autoscaling:master-13",
		Ports: []corev1.ContainerPort{{ContainerPort: 8000, Protocol: "TCP"}},
		Resources: corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse("300Mi"),
			},
			Requests: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("300Mi"),
			},
		},
		Env: []corev1.EnvVar{
			{
				Name:  metricNameToEnv(container.MetricName),
				Value: strconv.FormatInt(container.MetricValue, 10),
			},
		},
	}
}

// metricNameToEnv converts a metric name like "queue-count" to an env var like "QUEUE_COUNT"
func metricNameToEnv(metric string) string {
	nodashes := strings.Replace(metric, "-", "_", 1)
	allcaps := strings.ToUpper(nodashes)
	return allcaps
}

func simplePodMetricHPA(deploymentName string, metricName string, metricTarget int64) *autoscaling.HorizontalPodAutoscaler {
	return podMetricHPA(deploymentName, map[string]int64{metricName: metricTarget})
}

func podMetricHPA(deploymentName string, metricTargets map[string]int64) *autoscaling.HorizontalPodAutoscaler {
	var minReplicas int32 = 1
	metrics := []autoscaling.MetricSpec{}
	metricName := ""
	for metric, target := range metricTargets {
		metrics = append(metrics, autoscaling.MetricSpec{
			Type: autoscaling.PodsMetricSourceType,
			Pods: &autoscaling.PodsMetricSource{
				MetricName:         metric,
				TargetAverageValue: *resource.NewQuantity(target, resource.DecimalSI),
			},
		})
		metricName = metric
	}
	return &autoscaling.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name: "custom-metrics-pods-hpa",
			Annotations: map[string]string{
				strings.Join([]string{"metric-config.pods", metricName, "json-path/json-key"}, "."): "$.queue_count",
				strings.Join([]string{"metric-config.pods", metricName, "json-path/path"}, "."):     "/metrics",
				strings.Join([]string{"metric-config.pods", metricName, "json-path/port"}, "."):     "8000",
			},
			Labels: map[string]string{
				"application": deploymentName,
			},
		},
		Spec: autoscaling.HorizontalPodAutoscalerSpec{
			Metrics:     metrics,
			MaxReplicas: 3,
			MinReplicas: &minReplicas,
			ScaleTargetRef: autoscaling.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       deploymentName,
			},
		},
	}
}

func rpsBasedHPA(deploymentName string, ingressName, ingressAPIVersion string, metricTarget int64) *autoscaling.HorizontalPodAutoscaler {
	return podHPA(deploymentName, ingressName, ingressAPIVersion, map[string]int64{"requests-per-second": metricTarget})
}

func podHPA(deploymentName string, ingressName, ingressAPIVersion string, metricTargets map[string]int64) *autoscaling.HorizontalPodAutoscaler {
	var minReplicas int32 = 1
	metrics := []autoscaling.MetricSpec{}
	for metric, target := range metricTargets {
		metrics = append(metrics, autoscaling.MetricSpec{
			Type: autoscaling.ObjectMetricSourceType,
			Object: &autoscaling.ObjectMetricSource{
				MetricName: metric,
				Target: autoscaling.CrossVersionObjectReference{
					APIVersion: ingressAPIVersion,
					Kind:       "Ingress",
					Name:       ingressName,
				},
				TargetValue:  *resource.NewQuantity(target, resource.DecimalSI),
				AverageValue: resource.NewQuantity(target, resource.DecimalSI),
			},
		})
	}

	return &autoscaling.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name: "custom-metrics-pods-hpa",
			Labels: map[string]string{
				"application": deploymentName,
			},
		},
		Spec: autoscaling.HorizontalPodAutoscalerSpec{
			Metrics:     metrics,
			MaxReplicas: 3,
			MinReplicas: &minReplicas,
			ScaleTargetRef: autoscaling.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       deploymentName,
			},
		},
	}
}
