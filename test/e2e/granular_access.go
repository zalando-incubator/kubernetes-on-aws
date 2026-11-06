package e2e

import (
	"context"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	imageutils "k8s.io/kubernetes/test/utils/image"
	admissionapi "k8s.io/pod-security-admission/api"

	authv1 "k8s.io/api/authentication/v1"
)

const (
	aaasTestUser        = "granular-access-test-user"
	aaasPositiveUser    = "granular-access-positive-test-user"
	aaasPositiveApp     = "granular-access-e2e-app"
	aaasAdminGroup      = "zalando:administrator"
	aaasEngineerGroup   = "zalando:engineer"
	aaasDenyMsgFragment = "could not determine a known identity for user"
	aaasApplicationKey  = "application"
)

// These tests verify unknown-identity denials and a test-only static ownership
// mapping on e2e clusters. The mapping does not replace normal AaaS ownership.
// The ConfigMap tests use server-side dry-run; the webhook is still invoked
// because it is registered with sideEffects=None.
var _ = Describe("Granular Access admission webhook [Zalando]", func() {
	f := framework.NewDefaultFramework("granular-access")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelBaseline

	var (
		ns         string
		bogusAppID string
	)

	BeforeEach(func() {
		ns = f.Namespace.Name
		bogusAppID = "e2e-nonexistent-app-" + string(uuid.NewUUID())[:8]
	})

	impersonatedClient := func(ctx context.Context, user string) (kubernetes.Interface, *rest.Config) {
		config := rest.CopyConfig(f.ClientConfig())
		config.Impersonate = rest.ImpersonationConfig{UserName: user, Groups: []string{aaasEngineerGroup}}
		cs, err := kubernetes.NewForConfig(config)
		framework.ExpectNoError(err, "creating impersonated client")

		review, err := cs.AuthenticationV1().SelfSubjectReviews().Create(ctx, &authv1.SelfSubjectReview{}, metav1.CreateOptions{})
		framework.ExpectNoError(err, "SelfSubjectReview failed for impersonated user %q", user)
		Expect(review.Status.UserInfo.Username).To(Equal(user))
		Expect(slices.Contains(review.Status.UserInfo.Groups, aaasEngineerGroup)).To(BeTrue())
		Expect(slices.Contains(review.Status.UserInfo.Groups, aaasAdminGroup)).To(BeFalse())
		framework.Logf("impersonated e2e identity: %q (groups: %v)", review.Status.UserInfo.Username, review.Status.UserInfo.Groups)
		return cs, config
	}

	dryRunConfigMap := func(ctx context.Context, cs kubernetes.Interface, name string, labels map[string]string) error {
		cm := createConfigMap(name, ns, labels, map[string]string{"k": "v"})
		_, err := cs.CoreV1().ConfigMaps(ns).Create(ctx, cm, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
		return err
	}

	It("denies a labeled resource for an unknown identity", func(ctx context.Context) {
		cs, _ := impersonatedClient(ctx, aaasTestUser)
		err := dryRunConfigMap(ctx, cs, "aaas-bogus-app", map[string]string{aaasApplicationKey: bogusAppID})
		Expect(err).To(HaveOccurred(), "unknown identity must be denied")
		Expect(err.Error()).To(ContainSubstring(aaasDenyMsgFragment))
		Expect(err.Error()).To(ContainSubstring(aaasTestUser))
	})

	It("allows the configured user to create and update their application only", func(ctx context.Context) {
		cs, _ := impersonatedClient(ctx, aaasPositiveUser)
		// This denial ensures that a missing webhook cannot make the positive
		// test pass and that the exception is scoped to the configured app.
		err := dryRunConfigMap(ctx, cs, "aaas-wrong-app", map[string]string{aaasApplicationKey: bogusAppID})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(aaasDenyMsgFragment))
		Expect(err.Error()).To(ContainSubstring(aaasPositiveUser))
		otherUser, _ := impersonatedClient(ctx, aaasTestUser)
		err = dryRunConfigMap(ctx, otherUser, "aaas-wrong-user", map[string]string{aaasApplicationKey: aaasPositiveApp})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(aaasDenyMsgFragment))
		Expect(err.Error()).To(ContainSubstring(aaasTestUser))

		err = dryRunConfigMap(ctx, cs, "aaas-owned-app", map[string]string{aaasApplicationKey: aaasPositiveApp})
		framework.ExpectNoError(err, "creating a ConfigMap for the mapped application")

		cm := createConfigMap("aaas-owned-update", ns, map[string]string{aaasApplicationKey: aaasPositiveApp}, map[string]string{"k": "v"})
		cm, err = f.ClientSet.CoreV1().ConfigMaps(ns).Create(ctx, cm, metav1.CreateOptions{})
		framework.ExpectNoError(err, "creating ConfigMap for update test")
		cm.Data["k"] = "updated"
		_, err = cs.CoreV1().ConfigMaps(ns).Update(ctx, cm, metav1.UpdateOptions{DryRun: []string{metav1.DryRunAll}})
		framework.ExpectNoError(err, "updating a ConfigMap for the mapped application")
	})

	It("denies deleting a resource for an unknown identity", func(ctx context.Context) {
		cs, _ := impersonatedClient(ctx, aaasTestUser)
		cm := createConfigMap("aaas-delete", ns, nil, map[string]string{"k": "v"})
		_, err := f.ClientSet.CoreV1().ConfigMaps(ns).Create(ctx, cm, metav1.CreateOptions{})
		framework.ExpectNoError(err, "creating ConfigMap for delete test")

		err = cs.CoreV1().ConfigMaps(ns).Delete(ctx, cm.Name, metav1.DeleteOptions{DryRun: []string{metav1.DryRunAll}})
		Expect(err).To(HaveOccurred(), "delete by an unknown identity must be denied")
		Expect(err.Error()).To(ContainSubstring(aaasDenyMsgFragment))
		Expect(err.Error()).To(ContainSubstring(aaasTestUser))
	})

	It("allows exec into a pod for the configured application", func(ctx context.Context) {
		_, config := impersonatedClient(ctx, aaasPositiveUser)
		labels := map[string]string{"app": "aaas-positive-exec", aaasApplicationKey: aaasPositiveApp}
		replicas := int32(1)
		deploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "aaas-positive-exec", Namespace: ns},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "aaas-positive-exec"}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: labels},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{
						Name: "test", Image: imageutils.GetE2EImage(imageutils.BusyBox), Command: []string{"sh", "-c", "sleep 3600"},
					}}},
				},
			},
		}
		_, err := f.ClientSet.AppsV1().Deployments(ns).Create(ctx, deploy, metav1.CreateOptions{})
		framework.ExpectNoError(err)
		var podName string
		err = wait.PollUntilContextTimeout(ctx, framework.Poll, framework.PodStartTimeout, true, func(ctx context.Context) (bool, error) {
			pods, err := f.ClientSet.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: "app=aaas-positive-exec"})
			if err != nil {
				return false, err
			}
			for _, pod := range pods.Items {
				if pod.Status.Phase == corev1.PodRunning {
					podName = pod.Name
					return true, nil
				}
			}
			return false, nil
		})
		framework.ExpectNoError(err, "waiting for the application pod")
		_, stderr, err := e2epod.ExecWithOptionsTCtx(f.TContext(ctx).WithRESTConfig(config), e2epod.ExecOptions{
			Command: []string{"true"}, Namespace: ns, PodName: podName, ContainerName: "test",
			CaptureStdout: true, CaptureStderr: true, Quiet: true,
		})
		framework.ExpectNoError(err, "exec into mapped application pod failed: %s", stderr)
	})

	It("denies pods/exec for an unknown identity (CONNECT)", func(ctx context.Context) {
		_, config := impersonatedClient(ctx, aaasTestUser)
		// Creating a pod with the bogus label as the caller would itself be
		// denied. The Deployment is unlabeled (allowed); its pods are created
		// by the controller manager, which is excluded from the webhook.
		labels := map[string]string{"app": "aaas-exec", aaasApplicationKey: bogusAppID}
		replicas := int32(1)
		deploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "aaas-exec", Namespace: ns},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "aaas-exec"}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: labels},
					Spec:       corev1.PodSpec{Containers: []corev1.Container{pauseContainer()}},
				},
			},
		}
		_, err := f.ClientSet.AppsV1().Deployments(ns).Create(ctx, deploy, metav1.CreateOptions{})
		framework.ExpectNoError(err)

		var podName string
		err = wait.PollUntilContextTimeout(ctx, framework.Poll, framework.PodStartTimeout, true, func(ctx context.Context) (bool, error) {
			pods, err := f.ClientSet.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: "app=aaas-exec"})
			if err != nil {
				return false, err
			}
			for _, p := range pods.Items {
				if p.Status.Phase == corev1.PodRunning {
					podName = p.Name
					return true, nil
				}
			}
			return false, nil
		})
		framework.ExpectNoError(err, "waiting for the controller-created pod to run")

		_, stderr, err := e2epod.ExecWithOptionsTCtx(f.TContext(ctx).WithRESTConfig(config), e2epod.ExecOptions{
			Command:            []string{"true"},
			Namespace:          ns,
			PodName:            podName,
			ContainerName:      "pause",
			CaptureStdout:      true,
			CaptureStderr:      true,
			Quiet:              true,
			PreserveWhitespace: false,
		})
		Expect(err).To(HaveOccurred(), "exec must be denied by the AaaS webhook")
		Expect(strings.Join([]string{err.Error(), stderr}, " ")).To(ContainSubstring(aaasDenyMsgFragment))
		Expect(strings.Join([]string{err.Error(), stderr}, " ")).To(ContainSubstring(aaasTestUser))
	})
})
