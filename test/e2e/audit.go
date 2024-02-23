package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zalando-incubator/kubernetes-on-aws/tests/e2e/utils"
	authnv1 "k8s.io/api/authentication/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	auditinternal "k8s.io/apiserver/pkg/apis/audit"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	jsonpatch "github.com/evanphx/json-patch"
	. "github.com/onsi/ginkgo/v2"
)

var (
	auditTestUser = authnv1.UserInfo{
		Username: "zalando-iam:zalando:service:stups_kubernetes",
		UID:      "zalando-iam:zalando:service:stups_kubernetes",
		Groups: []string{
			"system:masters",
			"zalando-iam:realm:services",
			"system:authenticated",
		},
	}
	patch, _ = json.Marshal(jsonpatch.Patch{})
)

var _ = describe("Audit", func() {
	f := framework.NewDefaultFramework("audit")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelBaseline
	var namespace string
	BeforeEach(func() {
		namespace = f.Namespace.Name
	})

	It("Should audit API calls to create, update, patch, delete pods. [Audit] [Zalando]", func() {
		pod := &apiv1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "audit-pod",
			},
			Spec: apiv1.PodSpec{
				Containers: []apiv1.Container{{
					Name:  "pause",
					Image: "container-registry.zalando.net/teapot/pause:3.4.1-master-18",
				}},
			},
		}
		updatePod := func(pod *apiv1.Pod) {}

		e2epod.NewPodClient(f).CreateSync(pod)

		e2epod.NewPodClient(f).Update(pod.Name, updatePod)

		_, err := e2epod.NewPodClient(f).Patch(context.TODO(), pod.Name, types.JSONPatchType, patch, metav1.PatchOptions{})
		framework.ExpectNoError(err, "failed to patch pod")

		e2epod.NewPodClient(f).DeleteSync(pod.Name, metav1.DeleteOptions{}, e2epod.DefaultPodDeletionTimeout)

		expectEvents(f, []utils.AuditEvent{
			{
				Level:             auditinternal.LevelRequest,
				Stage:             auditinternal.StageResponseComplete,
				RequestURI:        fmt.Sprintf("/api/v1/namespaces/%s/pods", namespace),
				Verb:              "create",
				Code:              201,
				User:              auditTestUser,
				Resource:          "pods",
				Namespace:         namespace,
				RequestObject:     true,
				AuthorizeDecision: "allow",
			}, {
				Level:             auditinternal.LevelRequest,
				Stage:             auditinternal.StageResponseComplete,
				RequestURI:        fmt.Sprintf("/api/v1/namespaces/%s/pods/audit-pod", namespace),
				Verb:              "update",
				Code:              200,
				User:              auditTestUser,
				Resource:          "pods",
				Namespace:         namespace,
				RequestObject:     true,
				AuthorizeDecision: "allow",
			}, {
				Level:             auditinternal.LevelRequest,
				Stage:             auditinternal.StageResponseComplete,
				RequestURI:        fmt.Sprintf("/api/v1/namespaces/%s/pods/audit-pod", namespace),
				Verb:              "patch",
				Code:              200,
				User:              auditTestUser,
				Resource:          "pods",
				Namespace:         namespace,
				RequestObject:     true,
				AuthorizeDecision: "allow",
			}, {
				Level:             auditinternal.LevelRequest,
				Stage:             auditinternal.StageResponseComplete,
				RequestURI:        fmt.Sprintf("/api/v1/namespaces/%s/pods/audit-pod", namespace),
				Verb:              "delete",
				Code:              200,
				User:              auditTestUser,
				Resource:          "pods",
				Namespace:         namespace,
				RequestObject:     true,
				AuthorizeDecision: "allow",
			},
		})
	})
})

func expectEvents(f *framework.Framework, expectedEvents []utils.AuditEvent) {
	// The default flush timeout is 30 seconds, therefore it should be enough to retry once
	// to find all expected events. However, we're waiting for 5 minutes to avoid flakes.
	pollingInterval := 30 * time.Second
	pollingTimeout := 5 * time.Minute
	err := wait.Poll(pollingInterval, pollingTimeout, func() (bool, error) {
		// Fetch the log stream.
		stream, err := f.ClientSet.CoreV1().RESTClient().Get().AbsPath("/logs/kube-audit.log").Stream(context.TODO())
		if err != nil {
			return false, err
		}
		defer stream.Close()
		missingReport, err := utils.CheckAuditLines(stream, expectedEvents, auditv1.SchemeGroupVersion)
		if err != nil {
			framework.Logf("Failed to observe audit events: %v", err)
		} else if len(missingReport.MissingEvents) > 0 {
			framework.Logf("Events %#v not found!", missingReport)
		}
		return len(missingReport.MissingEvents) == 0, nil
	})
	framework.ExpectNoError(err, "after %v failed to observe audit events", pollingTimeout)
}
