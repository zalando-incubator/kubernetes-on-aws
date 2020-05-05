package e2e

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/zalando-incubator/kubernetes-on-aws/tests/e2e/utils"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	auditinternal "k8s.io/apiserver/pkg/apis/audit"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"

	jsonpatch "github.com/evanphx/json-patch"
	. "github.com/onsi/ginkgo"
)

var (
	auditTestUser = "kubelet"
	patch, _      = json.Marshal(jsonpatch.Patch{})
)

var _ = framework.KubeDescribe("Audit", func() {
	f := framework.NewDefaultFramework("audit")
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
					Image: "registry.opensource.zalan.do/teapot/pause-amd64:3.1",
				}},
			},
		}
		updatePod := func(pod *apiv1.Pod) {}

		f.PodClient().CreateSync(pod)

		f.PodClient().Update(pod.Name, updatePod)

		_, err := f.PodClient().Patch(pod.Name, types.JSONPatchType, patch)
		framework.ExpectNoError(err, "failed to patch pod")

		f.PodClient().DeleteSync(pod.Name, &metav1.DeleteOptions{}, framework.DefaultPodDeletionTimeout)

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
		stream, err := f.ClientSet.CoreV1().RESTClient().Get().AbsPath("/logs/kube-audit.log").Stream()
		if err != nil {
			return false, err
		}
		defer stream.Close()
		missingReport, err := utils.CheckAuditLines(stream, expectedEvents, auditv1.SchemeGroupVersion)
		if err != nil {
			e2elog.Logf("Failed to observe audit events: %v", err)
		} else if len(missingReport.MissingEvents) > 0 {
			e2elog.Logf("Events %#v not found!", missingReport)
		}
		return len(missingReport.MissingEvents) == 0, nil
	})
	framework.ExpectNoError(err, "after %v failed to observe audit events", pollingTimeout)
}
