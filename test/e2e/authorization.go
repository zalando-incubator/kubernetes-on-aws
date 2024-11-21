package e2e

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

var (
	allGroups = [][]string{
		{"FooBar"},
		{"ReadOnly"},
		{"PowerUser"},
		{"Emergency"},
		{"Manual"},
		{"system:serviceaccounts:kube-system"},
		{"CollaboratorEmergency"},
		{"CollaboratorManual"},
		{"Collaborator24x7"},
		{"CollaboratorPowerUser"},
		{"Administrator"},
	}
)

var _ = g.Describe("Authorization [RBAC] [Zalando]", func() {
	var cs kubernetes.Interface

	f := framework.NewDefaultFramework("authorization")

	// Initialise the clientset before each test
	g.BeforeEach(func() {
		cs = f.ClientSet
	})

	// Test cases for all groups of users
	g.Context("For all groups", func() {
		var tc testCase
		g.BeforeEach(func() {
			tc.data.groups = allGroups
			tc.data.users = []string{"test-user"}
		})
		g.When("the verb is impersonate", func() {
			g.BeforeEach(func() {
				tc.data.verbs = []string{"impersonate"}
			})

			g.It("should deny access for users and groups", func() {
				// This is safe to do since the BeforeEach block
				// will clear these values for other specs.
				// https://onsi.github.io/ginkgo/#organizing-specs-with-container-nodes
				tc.data.resources = []string{"users", "groups"}
				tc.run(context.TODO(), cs)
				output := tc.output
				gomega.Expect(output.denied).To(gomega.BeTrue())

			})
			g.It("should deny access for service accounts", func() {
				tc.data.resources = []string{"serviceaccounts"}
				tc.data.namespaces = []string{"", "teapot", "kube-system"}
				tc.run(context.TODO(), cs)
				output := tc.output
				gomega.Expect(output.denied).To(gomega.BeTrue())
			})
		})
		g.When("the verb is escalate", func() {
			g.BeforeEach(func() {
				tc.data.verbs = []string{"escalate"}
			})

			g.It("should deny access for cluster roles", func() {
				tc.data.resources = []string{"rbac.authorization.k8s.io/clusterrole"}
				tc.run(context.TODO(), cs)
				output := tc.output
				gomega.Expect(output.denied).To(gomega.BeTrue())
			})
			g.It("should deny access for roles in all namespaces", func() {
				tc.data.resources = []string{"rbac.authorization.k8s.io/role"}
				tc.data.namespaces = []string{"", "teapot", "kube-system"}
				tc.run(context.TODO(), cs)
				output := tc.output
				gomega.Expect(output.denied).To(gomega.BeTrue())
			})
		})
	})

	g.Context("For ReadOnly group", func() {
		var tc testCase
		g.BeforeEach(func() {
			tc.data.groups = [][]string{{"ReadOnly"}}
			tc.data.users = []string{"test-user"}
		})
		g.When("the resource is a Secret", func() {
			g.BeforeEach(func() {
				tc.data.resources = []string{"secrets"}
			})
			g.It("should deny access in all namespaces", func() {
				tc.data.verbs = []string{"get", "list", "watch", "create", "update", "delete", "patch"}
				tc.data.namespaces = []string{"", "teapot", "kube-system"}
				tc.run(context.TODO(), cs)
				output := tc.output
				gomega.Expect(output.denied).To(gomega.BeTrue())
			})
		})
		g.When("the resource is not a Secret resource", func() {
			g.BeforeEach(func() {
				tc.data.resources = []string{
					"pods",
					"apps/deployments",
					"apps/daemonsets",
					"apps/statefulsets",
					"apps/deployments/scale",
					"apps/statefulsets/scale",
					"services",
					"persistentvolumes",
					"persistentvolumeclaims",
					"configmaps",
				}
				tc.data.namespaces = []string{"", "teapot", "kube-system"}
			})
			g.It("should allow read access in all namespaces", func() {
				tc.data.verbs = []string{"get", "list", "watch"}
				tc.run(context.TODO(), cs)
				output := tc.output
				gomega.Expect(output.allowed).To(gomega.BeTrue(),
					"Reason: %v", output.reason)
			})
			g.It("should deny write access in all namespaces", func() {
				tc.data.verbs = []string{"create", "update", "delete", "patch"}
				tc.run(context.TODO(), cs)
				output := tc.output
				gomega.Expect(output.denied).To(gomega.BeTrue())
			})
		})
		g.When("the resource is a global resource", func() {
			g.BeforeEach(func() {
				tc.data.resources = []string{
					"namespaces",
					"nodes",
					"rbac.authorization.k8s.io/clusterroles",
					"storage.k8s.io/storageclasses",
					"apiextensions.k8s.io/customresourcedefinitions",
				}
				g.It("should allow read access", func() {
					tc.data.verbs = []string{"get", "list", "watch"}
					tc.run(context.TODO(), cs)
					output := tc.output
					gomega.Expect(output.allowed).To(gomega.BeTrue(),
						"Reason: %v", output.reason)
				})
				g.It("should deny write access", func() {
					tc.data.verbs = []string{"create", "update", "delete", "patch"}
					tc.run(context.TODO(), cs)
					output := tc.output
					gomega.Expect(output.denied).To(gomega.BeTrue())
				})
			})
		})
	})

	g.Context("For PowerUser, Manual and Emergency groups", func() {
		var tc testCase
		g.BeforeEach(func() {
			tc.data.groups = [][]string{
				{"PowerUser"},
				{"Manual"},
				{"Emergency"},
			}
			tc.data.users = []string{"test-user"}
		})

		g.It("should deny read access to Secrets in kube-system and visibility namespaces", func() {
			tc.data.resources = []string{"secrets"}
			tc.data.namespaces = []string{"kube-system", "visibility"}
			tc.data.verbs = []string{"get", "list", "watch"}
			tc.run(context.TODO(), cs)
			output := tc.output
			gomega.Expect(output.denied).To(gomega.BeTrue())
		})

		g.It("should deny write access to Nodes", func() {
			tc.data.resources = []string{"nodes"}
			tc.data.verbs = []string{"create", "update", "delete", "patch"}
			tc.run(context.TODO(), cs)
			output := tc.output
			gomega.Expect(output.denied).To(gomega.BeTrue())
		})

		g.It("should deny write access to DaemonSets", func() {
			tc.data.resources = []string{"apps/daemonsets"}
			tc.data.verbs = []string{"create", "update", "delete", "patch"}
			tc.run(context.TODO(), cs)
			output := tc.output
			gomega.Expect(output.denied).To(gomega.BeTrue())
		})

		// TODO: Double check if the original test case is correct
		g.It("should allow deleting CRDs", func() {
			tc.data.resources = []string{"apiextensions.k8s.io/customresourcedefinitions"}
			tc.data.verbs = []string{"delete"}
			tc.run(context.TODO(), cs)
			output := tc.output
			gomega.Expect(output.allowed).To(gomega.BeTrue())
		})

		g.It("should deny deleting kube-system or visibility namespaces", func() {
			tc.data.resources = []string{"namespaces"}
			tc.data.namespaces = []string{"kube-system", "visibility"}
			tc.data.verbs = []string{"delete"}
			tc.run(context.TODO(), cs)
			output := tc.output
			gomega.Expect(output.denied).To(gomega.BeTrue())
		})

		g.When("the resource is a namespaced resource", func() {
			g.BeforeEach(func() {
				tc.data.resources = []string{
					"pods",
					"apps/deployments",
					"apps/statefulsets",
					"apps/deployments/scale",
					"apps/statefulsets/scale",
					"services",
					"persistentvolumes",
					"persistentvolumeclaims",
					"configmaps",
				}
				tc.data.verbs = []string{"create", "update", "delete", "patch"}
			})
			g.It("should deny write access in kube-system and visibility namespaces", func() {
				tc.data.namespaces = []string{"kube-system", "visibility"}
				tc.run(context.TODO(), cs)
				output := tc.output
				gomega.Expect(output.denied).To(gomega.BeTrue())
			})
			g.It("should allow write access in namespaces other than kube-system and visibility", func() {
				tc.data.namespaces = []string{"", "teapot"}
				tc.run(context.TODO(), cs)
				output := tc.output
				gomega.Expect(output.allowed).To(gomega.BeTrue(),
					"Reason: %v", output.reason)
			})
		})
		g.When("the resource is a global resource", func() {
			g.BeforeEach(func() {
				tc.data.verbs = []string{"create", "update", "delete", "patch"}
			})
			g.It("should deny write access to Nodes", func() {
				tc.data.resources = []string{"nodes"}
				tc.run(context.TODO(), cs)
				output := tc.output
				gomega.Expect(output.denied).To(gomega.BeTrue())
			})
			g.It("should allow write access to resources other than Nodes", func() {
				tc.data.resources = []string{
					"namespaces",
					"storage.k8s.io/storageclasses",
					"apiextensions.k8s.io/customresourcedefinitions",
				}
				tc.run(context.TODO(), cs)
				output := tc.output
				gomega.Expect(output.allowed).To(gomega.BeTrue(),
					"Reason: %v", output.reason)
			})
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
