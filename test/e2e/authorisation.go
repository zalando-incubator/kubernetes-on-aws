package e2e

import (
	g "github.com/onsi/ginkgo/v2"
)

var _ = g.Describe("Authorisation [RBAC] [Zalando]", func() {

	g.Context("For all groups", func() {
		g.When("the verb is impersonate", func() {
			g.It("should deny access for users", func() {})
			g.It("should deny access for service accounts", func() {})
		})
		g.When("the verb is escalate", func() {
			g.It("should deny access for cluster roles", func() {})
			g.It("should deny access for roles in all namespaces", func() {})
		})
	})

	g.Context("For ReadOnly group", func() {
		g.When("the resource is a Secret", func() {
			g.It("should deny read access in all namespaces", func() {})
		})
		g.When("the resource is not a Secret resource", func() {
			g.It("should allow read access in all namespaces", func() {})
			g.It("should deny write access in all namespaces", func() {})
		})
		g.When("the resource is a global resource", func() {
			g.It("should allow read access", func() {})
			g.It("should deny write access", func() {})
		})
	})

	g.Context("For PowerUser, Manual and Emergency groups", func() {

		g.It("should deny read access to Secrets in kube-system and visibility namespaces", func() {})
		g.It("should deny write access to Nodes", func() {})
		g.It("should deny write access to DaemonSets", func() {})
		g.It("should deny deleting CRDs", func() {})
		g.It("should deny deleting kube-system or visibility namespaces", func() {})

		g.When("the resource is a namespaced resource", func() {
			g.It("should deny write access in kube-system and visibility namespaces", func() {})
			g.It("should allow write access in namespaces other than kube-system and visibility", func() {})
		})
		g.When("the resource is a global resource", func() {
			g.It("should deny access to Nodes", func() {})
			g.It("should allow access to resources other than Nodes", func() {})
		})
	})

	g.Context("For CollaboratorPowerUser, CollaboratorManual and CollaboratorEmergency groups", func() {
		g.When("the resource is a Secret", func() {
			g.It("should allow read access to visibility namespace", func() {})
			g.It("should deny read access to kube-system namespace", func() {})
		})

		g.It("should deny write access to Nodes", func() {})
		g.It("should allow write access to DaemonSets", func() {})
		g.It("should allow deletion of CRDs", func() {})
		g.It("should deny deletion of kube-system or visibility namespaces", func() {})

		g.When("the resource is a namespaced resource", func() {
			g.It("should deny write access in kube-system namespace", func() {})
			g.It("should allow write access in namespaces other than kube-system", func() {})
		})

		g.When("the resource is a global resource", func() {
			g.It("should deny access to Nodes", func() {})
			g.It("should allow access to resources other than Nodes", func() {})
		})
	})

	g.Context("For system users", func() {
		g.When("the user is kubelet", func() {
			g.It("should allow to get Pods", func() {})
		})

		g.When("the service account is daemonset-controller", func() {
			g.It("should allow to update DaemonSet status subresource", func() {})
			g.It("should allow to update DaemonSet finalizers", func() {})
			g.It("should allow to create Pods", func() {})
		})

		g.When("the service account is default", func() {
			g.It("should deny to list StatefulSets when in default namespace", func() {})
			g.It("should deny to list StatefulSets when in non-default namespace", func() {})
		})

		g.When("the service account is persistent-volume-binder", func() {
			g.It("should allow to update PersistentVolumeClaims", func() {})
			g.It("should allow to create PersistentVolumes", func() {})

		})

		g.When("the service account is aws-cloud-provider", func() {
			g.It("should allow to patch Nodes", func() {})
		})

		g.When("the service account is api-monitoring-controller", func() {
			g.It("should allow to update the skipper-default-filters ConfigMap in kube-system namespace", func() {})
			g.It("should deny to update ConfigMaps other than skipper-default-filters", func() {})
		})

		g.When("the user is k8sapi_credentials-provider", func() {
			g.It("should allow to get Secrets in kube-system namespace", func() {})
		})

		g.When("the user is stups_cdp-controller", func() {
			g.It("should deny access to Secrets in kube-system namespace", func() {})
		})

	})

	g.Context("For administrators", func() {
		g.It("should allow read access to resources other than Secrets in kube-system namespace", func() {})
		g.It("should allow write access to resources other than Secrets in kube-system namespace", func() {})
		g.It("should allow read access to Secrets in kube-system namespace", func() {})
		g.It("should allow read access to Secrets in namespaces other than kube-system", func() {})
		g.It("should allow write access to namespaces other than kube-system", func() {})
		g.It("should allow to proxy", func() {})
		g.It("should allow write access to DaemonSets", func() {})
	})

})
