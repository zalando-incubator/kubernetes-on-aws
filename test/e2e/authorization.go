package e2e

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	g "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
	testutil "k8s.io/kubernetes/test/utils"
	admissionapi "k8s.io/pod-security-admission/api"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"
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

	// "secrets" are not included as they have their own set of test cases.
	namespacedResources = []string{
		"pods",
		"apps/deployments",
		"apps/statefulsets",
		"apps/deployments/scale",
		"apps/statefulsets/scale",
		"services",
		"persistentvolumeclaims",
		"configmaps",
	}

	// "nodes" are not included as they have their own set of test cases.
	globalResources = []string{
		"namespaces",
		"rbac.authorization.k8s.io/clusterroles",
		"storage.k8s.io/storageclasses",
		"apiextensions.k8s.io/customresourcedefinitions",
	}
	// a slice of "get", "list", "watch" verbs
	readOperations = []string{"get", "list", "watch"}

	// a slice of "create", "update", "delete", "patch" verbs
	writeOperations = []string{"create", "update", "delete", "patch"}

	// a slice of all operations
	allOperations = append(readOperations, writeOperations...)

	// a slice representing all namespaces with respect to the test cases
	// "default" is the default namespace
	// "teapot" is a random namespace
	// "visibility" is a namespace where collaborators will have access
	// "kube-system" is a namespace where only administrators will have access
	allNamespaces = []string{"default", "teapot", "visibility", "kube-system"}
)

var _ = g.Describe("Authorization [RBAC] [Zalando]", func() {
	var cs kubernetes.Interface

	f := framework.NewDefaultFramework("authorization")

	g.BeforeEach(func() {
		cs = f.ClientSet
	})

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
				tc.run(context.TODO(), cs, false)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})
			g.It("should deny access for service accounts", func() {
				tc.data.resources = []string{"serviceaccounts"}
				tc.data.namespaces = allNamespaces
				tc.run(context.TODO(), cs, false)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})
		})
		g.When("the verb is escalate", func() {
			g.BeforeEach(func() {
				tc.data.verbs = []string{"escalate"}
			})

			g.It("should deny access for cluster roles", func() {
				tc.data.resources = []string{"rbac.authorization.k8s.io/clusterrole"}
				tc.run(context.TODO(), cs, false)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})
			g.It("should deny access for roles in all namespaces", func() {
				tc.data.resources = []string{"rbac.authorization.k8s.io/role"}
				tc.data.namespaces = allNamespaces
				tc.run(context.TODO(), cs, false)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
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
				tc.data.verbs = allOperations
				tc.data.namespaces = allNamespaces
				tc.run(context.TODO(), cs, false)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})
		})
		g.When("the resource is not a Secret resource", func() {
			g.BeforeEach(func() {
				tc.data.resources = namespacedResources
				tc.data.namespaces = allNamespaces
			})
			g.It("should allow read access in all namespaces", func() {
				tc.data.verbs = readOperations
				tc.run(context.TODO(), cs, true)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})
			g.It("should deny write access in all namespaces", func() {
				tc.data.verbs = writeOperations
				tc.run(context.TODO(), cs, false)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})
		})
		g.When("the resource is a global resource", func() {
			g.BeforeEach(func() {
				tc.data.resources = append(globalResources, "nodes")
				g.It("should allow read access", func() {
					tc.data.verbs = readOperations
					tc.run(context.TODO(), cs, true)
					gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
				})
				g.It("should deny write access", func() {
					tc.data.verbs = writeOperations
					tc.run(context.TODO(), cs, false)
					gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
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
			tc.data.verbs = readOperations
			tc.run(context.TODO(), cs, false)
			gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
		})

		g.It("should allow read access to Secrets in namespaces other than kube-system and visibility", func() {
			tc.data.resources = []string{"secrets"}
			// The namespace must exist for the test case to pass, otherwise access
			// remains undecided.
			tc.data.namespaces = []string{"default"}
			tc.data.verbs = readOperations
			tc.run(context.TODO(), cs, true)
			gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
		})

		g.It("should deny write access to Nodes", func() {
			tc.data.resources = []string{"nodes"}
			tc.data.verbs = writeOperations
			tc.run(context.TODO(), cs, false)
			gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
		})

		g.It("should deny write access to DaemonSets", func() {
			tc.data.resources = []string{"apps/daemonsets"}
			tc.data.verbs = writeOperations
			tc.run(context.TODO(), cs, false)
			gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
		})

		g.It("should allow deleting CRDs", func() {
			tc.data.resources = []string{"apiextensions.k8s.io/customresourcedefinitions"}
			tc.data.verbs = []string{"delete"}
			tc.run(context.TODO(), cs, true)
			gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
		})

		g.It("should deny deleting kube-system or visibility namespaces", func() {
			g.Skip("handled by admission-controller")
		})

		g.When("the resource is a namespaced resource", func() {
			g.BeforeEach(func() {
				tc.data.resources = namespacedResources
				tc.data.verbs = writeOperations
			})
			g.It("should deny write access in kube-system and visibility namespaces", func() {
				g.Skip("handled by admission-controller")
			})
			g.It("should allow write access in namespaces other than kube-system and visibility", func() {
				g.Skip("handled by admission-controller")
			})
		})
		g.When("the resource is a global resource", func() {
			g.BeforeEach(func() {
				tc.data.verbs = writeOperations
			})
			g.It("should deny write access to Nodes", func() {
				tc.data.resources = []string{"nodes"}
				tc.run(context.TODO(), cs, false)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})
			g.It("should allow write access to resources other than Nodes", func() {
				tc.data.resources = globalResources
				tc.run(context.TODO(), cs, true)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})
		})
	})

	g.Context("For CollaboratorPowerUser, CollaboratorManual and CollaboratorEmergency groups", func() {
		var tc testCase
		g.BeforeEach(func() {
			tc.data.groups = [][]string{
				// Collaborator groups can escalate privileges to their respective groups
				// so, we need to include the respective group in the list as well.
				{"CollaboratorPowerUser", "PowerUser"},
				{"CollaboratorManual", "Manual"},
				{"CollaboratorEmergency", "Emergency"},
			}
			tc.data.users = []string{"test-user"}
		})

		g.When("the resource is a Secret", func() {
			g.BeforeEach(func() {
				tc.data.resources = []string{"secrets"}
				tc.data.verbs = readOperations
			})

			g.It("should allow read access to visibility namespace", func() {
				tc.data.namespaces = []string{"visibility"}
				tc.run(context.TODO(), cs, true)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})
			g.It("should deny read access to kube-system namespace", func() {
				tc.data.namespaces = []string{"kube-system"}
				tc.run(context.TODO(), cs, false)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})
		})

		g.It("should deny write access to Nodes", func() {
			tc.data.resources = []string{"nodes"}
			tc.data.verbs = writeOperations
			tc.run(context.TODO(), cs, false)
			gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
		})
		g.It("should allow write access to DaemonSets", func() {
			tc.data.resources = []string{"apps/daemonsets"}
			tc.data.verbs = writeOperations
			tc.data.namespaces = []string{"visibility"}
			tc.run(context.TODO(), cs, true)
			gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
		})
		g.It("should allow deletion of CRDs", func() {
			tc.data.resources = []string{"apiextensions.k8s.io/customresourcedefinitions"}
			tc.data.verbs = []string{"delete"}
			tc.run(context.TODO(), cs, true)
			gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
		})
		g.It("should deny deletion of kube-system or visibility namespaces", func() {
			g.Skip("handled by admission-controller")
		})

		g.When("the resource is a namespaced resource", func() {
			g.BeforeEach(func() {
				tc.data.resources = namespacedResources
				tc.data.verbs = writeOperations
			})
			g.It("should deny write access in kube-system namespace", func() {
				g.Skip("handled by admission-controller")
			})
			g.It("should allow write access in namespaces other than kube-system", func() {
				tc.data.namespaces = []string{"default", "teapot"}
				tc.run(context.TODO(), cs, true)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})
		})

		g.When("the resource is a global resource", func() {
			g.BeforeEach(func() {
				tc.data.verbs = writeOperations
			})
			g.It("should deny access to Nodes", func() {
				tc.data.resources = []string{"nodes"}
				tc.run(context.TODO(), cs, false)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})
			g.It("should allow access to resources other than Nodes", func() {
				tc.data.resources = globalResources
				tc.run(context.TODO(), cs, true)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})
		})
	})

	g.Context("For system users", func() {
		var tc testCase

		g.When("the user is kubelet", func() {
			g.BeforeEach(func() {
				tc.data.groups = [][]string{{"system:masters"}}
				tc.data.users = []string{"kubelet"}
			})
			g.It("should allow to get Pods", func() {
				tc.data.resources = []string{"pods"}
				tc.data.verbs = []string{"get"}
				tc.data.namespaces = []string{"teapot"}
				tc.run(context.TODO(), cs, true)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})
		})

		g.When("the service account is daemonset-controller", func() {
			g.BeforeEach(func() {
				tc.data.users = []string{"system:serviceaccount:kube-system:daemon-set-controller"}
				tc.data.groups = [][]string{{"system:serviceaccounts:kube-system"}}
			})
			g.It("should allow to update DaemonSet status subresource", func() {
				tc.data.resources = []string{"apps/daemonsets/status"}
				tc.data.verbs = []string{"update"}
				tc.run(context.TODO(), cs, true)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})
			g.It("should allow to update DaemonSet finalizers", func() {
				tc.data.resources = []string{"apps/daemonsets/finalizers"}
				tc.data.verbs = []string{"update"}
				tc.run(context.TODO(), cs, true)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})
			// TODO: Need to verify in the original tests if this is a permission on
			// the controller-manager or the daemonset-controller.
			// g.It("should allow to create Pods", func() {})
		})

		g.When("the service account is the default service account", func() {
			g.BeforeEach(func() {
				tc.data.users = []string{"system:serviceaccount:default:default", "system:serviceaccount:non-default:default"}
			})
			g.It("should deny to list StatefulSets", func() {
				tc.data.resources = []string{"apps/statefulsets"}
				tc.data.verbs = []string{"list"}
				tc.run(context.TODO(), cs, false)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})
		})

		g.When("the service account is persistent-volume-binder", func() {
			g.BeforeEach(func() {
				tc.data.users = []string{"system:serviceaccount:kube-system:persistent-volume-binder"}
				tc.data.groups = [][]string{{"system:serviceaccounts:kube-system"}}
				tc.data.namespaces = []string{"kube-system"}
			})
			g.It("should allow to update PersistentVolumeClaims", func() {
				tc.data.resources = []string{"persistentvolumeclaims"}
				tc.data.verbs = []string{"update"}
				tc.run(context.TODO(), cs, true)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})
			g.It("should allow to create PersistentVolumes", func() {
				tc.data.resources = []string{"persistentvolumes"}
				tc.data.verbs = []string{"create"}
				tc.run(context.TODO(), cs, true)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})

		})

		g.When("the service account is aws-cloud-provider", func() {
			g.BeforeEach(func() {
				tc.data.users = []string{"system:serviceaccount:kube-system:aws-cloud-provider"}
				tc.data.groups = [][]string{{"system:serviceaccounts:kube-system"}}
			})
			g.It("should allow to patch Nodes", func() {
				tc.data.resources = []string{"nodes"}
				tc.data.verbs = []string{"patch"}
				tc.run(context.TODO(), cs, true)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})
		})

		g.When("the service account is api-monitoring-controller", func() {
			g.BeforeEach(func() {
				tc.data.users = []string{"system:serviceaccount:api-infrastructure:api-monitoring-controller"}
			})
			g.When("the namespace is kube-system", func() {
				g.BeforeEach(func() {
					tc.data.namespaces = []string{"kube-system"}
				})
				g.It("should allow to update 'skipper-default-filters' ConfigMap", func() {
					tc.data.resources = []string{"configmaps"}
					tc.data.verbs = []string{"update"}
					tc.data.names = []string{"skipper-default-filters"}
					tc.run(context.TODO(), cs, true)
					gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
				})
				g.It("should deny to update any other ConfigMap", func() {
					tc.data.resources = []string{"configmaps"}
					tc.data.verbs = []string{"update"}
					// Technically, this should result in access undecided because we allow
					// access to 'skipper-default-filters' ConfigMap only and we haven't
					// specified a resource name in the test case.
					// We consider access undecided cases also as denied.
					tc.run(context.TODO(), cs, false)
					gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
				})
			})
		})

		g.When("the user is k8sapi_credentials-provider", func() {
			g.BeforeEach(func() {
				tc.data.users = []string{"zalando-iam:zalando:service:k8sapi_credentials-provider"}
				tc.data.resources = []string{"secrets"}
				tc.data.namespaces = []string{"kube-system"}
			})
			g.It("should not allow to delete secrets in kube-system namespace", func() {
				tc.data.verbs = []string{"delete"}
				tc.run(context.TODO(), cs, false)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})
			g.It("should allow all non-delete operations on secrets in kube-system namespace", func() {
				tc.data.verbs = []string{"get", "list", "watch", "create", "update", "patch"}
				tc.run(context.TODO(), cs, true)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})
		})

		g.When("the user is stups_cdp-controller", func() {
			g.BeforeEach(func() {
				tc.data.users = []string{"zalando-iam:zalando:service:stups_cdp-controller"}
			})
			g.When("the namespace is kube-system", func() {
				g.BeforeEach(func() {
					tc.data.namespaces = []string{"kube-system"}
				})
				g.It("should deny to get Secrets", func() {
					tc.data.resources = []string{"secrets"}
					tc.data.verbs = []string{"get"}
					tc.run(context.TODO(), cs, false)
					gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
				})
			})
		})

	})

	g.Context("For administrators", func() {
		var tc testCase
		g.BeforeEach(func() {
			tc.data.groups = [][]string{{"system:masters"}}
			tc.data.users = []string{"nmalik"}
		})

		g.When("namespace is kube-system", func() {
			g.BeforeEach(func() {
				tc.data.namespaces = []string{"kube-system"}
			})
			g.When("the resource is a Secret", func() {
				g.BeforeEach(func() {
					tc.data.resources = []string{"secrets"}
				})
				g.It("should allow read and write access", func() {
					tc.data.verbs = allOperations
					tc.run(context.TODO(), cs, true)
					gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
				})
			})

			g.When("the resource is not a Secret", func() {
				g.BeforeEach(func() {
					tc.data.resources = namespacedResources
				})
				g.It("should allow read and write access", func() {
					tc.data.verbs = allOperations
					tc.run(context.TODO(), cs, true)
					gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
				})
			})
		})

		g.When("namespace is not kube-system", func() {
			g.BeforeEach(func() {
				tc.data.namespaces = []string{"teapot"}
			})

			g.It("should allow to proxy", func() {
				tc.data.verbs = []string{"proxy"}
				tc.run(context.TODO(), cs, true)
				gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
			})

			g.When("the resource is a Secret", func() {
				g.BeforeEach(func() {
					tc.data.resources = []string{"secrets"}
				})
				g.It("should allow read access", func() {
					tc.data.verbs = readOperations
					tc.run(context.TODO(), cs, true)
					gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
				})
			})
			g.When("the resource is not a Secret", func() {
				g.BeforeEach(func() {
					tc.data.resources = namespacedResources
				})
				g.It("should allow write access", func() {
					tc.data.verbs = writeOperations
					tc.run(context.TODO(), cs, true)
					gomega.Expect(tc.output.passed).To(gomega.BeTrue(), tc.output.String())
				})
			})
		})
	})
})

var _ = g.Describe("Authorization via admission-controller [RBAC] [Zalando]", func() {
	var (
		awsAccountID string
		eksCluster   *types.Cluster
	)

	f := framework.NewDefaultFramework("authorization")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelBaseline

	g.BeforeEach(func() {
		cfg, err := config.LoadDefaultConfig(context.Background())
		framework.ExpectNoError(err)

		awsAccountID, err = getAWSAccountID(context.Background(), cfg)
		framework.ExpectNoError(err)

		eksCluster, err = getEKSCluster(context.Background(), cfg, f.ClientConfig())
		framework.ExpectNoError(err)
	})

	g.Context("for namespaced resources", func() {
		var (
			systemResource       *corev1.Pod
			collaboratorResource *corev1.Pod
			nonSystemResource    *corev1.Pod
		)

		g.BeforeEach(func() {
			var err error

			nonSystemResource, err = createPod(context.Background(), f.ClientSet, f.Namespace.Name, nil)
			framework.ExpectNoError(err)

			collaboratorResource, err = createPod(context.Background(), f.ClientSet, "visibility", nil)
			framework.ExpectNoError(err)

			systemResource, err = createPod(context.Background(), f.ClientSet, "kube-system", map[string]string{"admission.zalando.org/infrastructure-component": "true"})
			framework.ExpectNoError(err)
		})

		g.Context("as admin user", func() {
			var client *kubernetes.Clientset

			g.BeforeEach(func() {
				var err error

				client, err = getAdminClient(eksCluster, awsAccountID)
				framework.ExpectNoError(err)
			})

			g.It("should allow write access in user namespace", func() {
				err := client.CoreV1().Pods(nonSystemResource.Namespace).Delete(context.Background(), nonSystemResource.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				framework.ExpectNoError(err, "failed to delete pod: %s in namespace: %s", nonSystemResource.Name, nonSystemResource.Namespace)
			})

			g.It("should allow write access in collaborator namespace", func() {
				err := client.CoreV1().Pods(collaboratorResource.Namespace).Delete(context.Background(), collaboratorResource.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				framework.ExpectNoError(err, "failed to delete pod: %s in namespace: %s", collaboratorResource.Name, collaboratorResource.Namespace)
			})

			g.It("should allow write access in system namespace", func() {
				err := client.CoreV1().Pods(systemResource.Namespace).Delete(context.Background(), systemResource.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				framework.ExpectNoError(err, "failed to delete pod: %s in namespace: %s", systemResource.Name, systemResource.Namespace)
			})
		})

		g.Context("as collaborator user", func() {
			var client *kubernetes.Clientset

			g.BeforeEach(func() {
				var err error

				client, err = getCollaboratorClient(eksCluster, awsAccountID)
				framework.ExpectNoError(err)
			})

			g.It("should allow write access in user namespace", func() {
				err := client.CoreV1().Pods(nonSystemResource.Namespace).Delete(context.Background(), nonSystemResource.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				framework.ExpectNoError(err, "failed to delete pod: %s in namespace: %s", nonSystemResource.Name, nonSystemResource.Namespace)
			})

			g.It("should allow write access in collaborator namespace", func() {
				err := client.CoreV1().Pods(collaboratorResource.Namespace).Delete(context.Background(), collaboratorResource.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				framework.ExpectNoError(err, "failed to delete pod: %s in namespace: %s", collaboratorResource.Name, collaboratorResource.Namespace)
			})

			g.It("should deny write access in system namespace", func() {
				err := client.CoreV1().Pods(systemResource.Namespace).Delete(context.Background(), systemResource.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("write operations are forbidden")))
			})
		})

		g.Context("as engineer user", func() {
			var client *kubernetes.Clientset

			g.BeforeEach(func() {
				var err error

				client, err = getEngineerClient(eksCluster, awsAccountID)
				framework.ExpectNoError(err)
			})

			g.It("should allow write access in user namespace", func() {
				err := client.CoreV1().Pods(nonSystemResource.Namespace).Delete(context.Background(), nonSystemResource.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				framework.ExpectNoError(err, "failed to delete pod: %s in namespace: %s", nonSystemResource.Name, nonSystemResource.Namespace)
			})

			g.It("should deny write access in collaborator namespace", func() {
				err := client.CoreV1().Pods(collaboratorResource.Namespace).Delete(context.Background(), collaboratorResource.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("write operations are forbidden")))
			})

			g.It("should deny write access in system namespace", func() {
				err := client.CoreV1().Pods(systemResource.Namespace).Delete(context.Background(), systemResource.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("write operations are forbidden")))
			})
		})

		g.Context("as read-only user", func() {
			var client *kubernetes.Clientset

			g.BeforeEach(func() {
				var err error

				client, err = getReadOnlyClient(eksCluster, awsAccountID)
				framework.ExpectNoError(err)
			})

			g.It("should deny write access in user namespace", func() {
				err := client.CoreV1().Pods(nonSystemResource.Namespace).Delete(context.Background(), nonSystemResource.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("write operations are forbidden")))
			})

			g.It("should deny write access in collaborator namespace", func() {
				err := client.CoreV1().Pods(collaboratorResource.Namespace).Delete(context.Background(), collaboratorResource.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("write operations are forbidden")))
			})

			g.It("should deny write access in system namespace", func() {
				err := client.CoreV1().Pods(systemResource.Namespace).Delete(context.Background(), systemResource.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("write operations are forbidden")))
			})
		})
	})

	g.Context("for global resources", func() {
		var (
			systemResource    *rbacv1.ClusterRole
			nonSystemResource *rbacv1.ClusterRole
		)

		g.BeforeEach(func() {
			var err error

			systemResource, err = createClusterRole(context.Background(), f.ClientSet, map[string]string{"admission.zalando.org/infrastructure-component": "true"})
			framework.ExpectNoError(err)

			nonSystemResource, err = createClusterRole(context.Background(), f.ClientSet, nil)
			framework.ExpectNoError(err)
		})

		g.Context("as admin user", func() {
			var client *kubernetes.Clientset

			g.BeforeEach(func() {
				var err error

				client, err = getAdminClient(eksCluster, awsAccountID)
				framework.ExpectNoError(err)
			})

			g.It("should allow write access for non-system resources", func() {
				err := client.RbacV1().ClusterRoles().Delete(context.Background(), nonSystemResource.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				framework.ExpectNoError(err, "failed to delete cluster role: %s", nonSystemResource.Name)
			})

			g.It("should allow write access for system resources", func() {
				err := client.RbacV1().ClusterRoles().Delete(context.Background(), systemResource.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				framework.ExpectNoError(err, "failed to delete cluster role: %s", systemResource.Name)
			})
		})

		g.Context("as collaborator user", func() {
			var client *kubernetes.Clientset

			g.BeforeEach(func() {
				var err error

				client, err = getCollaboratorClient(eksCluster, awsAccountID)
				framework.ExpectNoError(err)
			})

			g.It("should allow write access for non-system resources", func() {
				err := client.RbacV1().ClusterRoles().Delete(context.Background(), nonSystemResource.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				framework.ExpectNoError(err, "failed to delete cluster role: %s", nonSystemResource.Name)
			})

			g.It("should deny write access for system resources", func() {
				err := client.RbacV1().ClusterRoles().Delete(context.Background(), systemResource.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("write operations are forbidden")))
			})

			// test specific namespaces
			g.It("should deny deletion of visibility namespace", func() {
				err := client.CoreV1().Namespaces().Delete(context.Background(), "visibility", metav1.DeleteOptions{DryRun: []string{"All"}})
				gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("write operations are forbidden")))
			})

			g.It("should deny deletion of kube-system namespace", func() {
				err := client.CoreV1().Namespaces().Delete(context.Background(), "kube-system", metav1.DeleteOptions{DryRun: []string{"All"}})
				gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("this namespace may not be deleted")))
			})
		})

		g.Context("as engineer user", func() {
			var client *kubernetes.Clientset

			g.BeforeEach(func() {
				var err error

				client, err = getEngineerClient(eksCluster, awsAccountID)
				framework.ExpectNoError(err)
			})

			g.It("should allow write access for non-system resources", func() {
				err := client.RbacV1().ClusterRoles().Delete(context.Background(), nonSystemResource.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				framework.ExpectNoError(err, "failed to delete cluster role: %s", nonSystemResource.Name)
			})

			g.It("should deny write access for system resources", func() {
				err := client.RbacV1().ClusterRoles().Delete(context.Background(), systemResource.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("write operations are forbidden")))
			})

			// test specific namespaces
			g.It("should deny deletion of visibility namespace", func() {
				err := client.CoreV1().Namespaces().Delete(context.Background(), "visibility", metav1.DeleteOptions{DryRun: []string{"All"}})
				gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("write operations are forbidden")))
			})

			g.It("should deny deletion of kube-system namespace", func() {
				err := client.CoreV1().Namespaces().Delete(context.Background(), "kube-system", metav1.DeleteOptions{DryRun: []string{"All"}})
				gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("this namespace may not be deleted")))
			})
		})

		g.Context("as read-only user", func() {
			var client *kubernetes.Clientset

			g.BeforeEach(func() {
				var err error

				client, err = getReadOnlyClient(eksCluster, awsAccountID)
				framework.ExpectNoError(err)
			})

			// why allow any write acess for read-only user?
			g.It("should allow write access for non-system resources", func() {
				err := client.RbacV1().ClusterRoles().Delete(context.Background(), nonSystemResource.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				framework.ExpectNoError(err, "failed to delete cluster role: %s", nonSystemResource.Name)
			})

			g.It("should deny write access for system resources", func() {
				err := client.RbacV1().ClusterRoles().Delete(context.Background(), systemResource.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("write operations are forbidden")))
			})
		})
	})

	g.Context("exec permission", func() {
		var (
			userPod     *corev1.Pod
			postgresPod *corev1.Pod
			systemPod   *corev1.Pod
		)

		g.BeforeEach(func() {
			var err error

			userPod, err = createPod(context.Background(), f.ClientSet, f.Namespace.Name, map[string]string{"application": "my-app"})
			framework.ExpectNoError(err)

			postgresPod, err = createPod(context.Background(), f.ClientSet, f.Namespace.Name, map[string]string{"application": "spilo"})
			framework.ExpectNoError(err)

			systemPod, err = createPod(context.Background(), f.ClientSet, "kube-system", map[string]string{"application": "my-app"})
			framework.ExpectNoError(err)
		})

		g.Context("as postgres administrator user", func() {
			var client *kubernetes.Clientset

			g.BeforeEach(func() {
				var err error

				client, err = getPostgresAdministratorClient(eksCluster, awsAccountID)
				framework.ExpectNoError(err)
			})

			g.It("should allow exec access for user pod", func() {
				result := client.CoreV1().RESTClient().Post().Namespace(userPod.Namespace).Resource("pods").Name(userPod.Name).SubResource("exec").Do(context.Background())
				gomega.Expect(result.Error()).To(gomega.MatchError(gomega.ContainSubstring("Upgrade request required")))
			})

			g.It("should allow exec access for postgres pod", func() {
				result := client.CoreV1().RESTClient().Post().Namespace(postgresPod.Namespace).Resource("pods").Name(postgresPod.Name).SubResource("exec").Do(context.Background())
				gomega.Expect(result.Error()).To(gomega.MatchError(gomega.ContainSubstring("Upgrade request required")))
			})

			g.It("should deny exec access for system pod", func() {
				result := client.CoreV1().RESTClient().Post().Namespace(systemPod.Namespace).Resource("pods").Name(systemPod.Name).SubResource("exec").Do(context.Background())
				gomega.Expect(result.Error()).To(gomega.MatchError(gomega.ContainSubstring("write operations are forbidden")))
			})
		})

		g.Context("as admin user", func() {
			var client *kubernetes.Clientset

			g.BeforeEach(func() {
				var err error

				client, err = getAdminClient(eksCluster, awsAccountID)
				framework.ExpectNoError(err)
			})

			g.It("should allow exec access for user pod", func() {
				result := client.CoreV1().RESTClient().Post().Namespace(userPod.Namespace).Resource("pods").Name(userPod.Name).SubResource("exec").Do(context.Background())
				gomega.Expect(result.Error()).To(gomega.MatchError(gomega.ContainSubstring("Upgrade request required")))
			})

			g.It("should allow exec access for postgres pod", func() {
				result := client.CoreV1().RESTClient().Post().Namespace(postgresPod.Namespace).Resource("pods").Name(postgresPod.Name).SubResource("exec").Do(context.Background())
				gomega.Expect(result.Error()).To(gomega.MatchError(gomega.ContainSubstring("Upgrade request required")))
			})

			g.It("should allow exec access for system pod", func() {
				result := client.CoreV1().RESTClient().Post().Namespace(systemPod.Namespace).Resource("pods").Name(systemPod.Name).SubResource("exec").Do(context.Background())
				gomega.Expect(result.Error()).To(gomega.MatchError(gomega.ContainSubstring("Upgrade request required")))
			})
		})

		g.Context("as read-only user", func() {
			var client *kubernetes.Clientset

			g.BeforeEach(func() {
				var err error

				client, err = getReadOnlyClient(eksCluster, awsAccountID)
				framework.ExpectNoError(err)
			})

			g.It("should allow exec access for user pod", func() {
				result := client.CoreV1().RESTClient().Post().Namespace(userPod.Namespace).Resource("pods").Name(userPod.Name).SubResource("exec").Do(context.Background())
				gomega.Expect(result.Error()).To(gomega.MatchError(gomega.ContainSubstring("Upgrade request required")))
			})

			g.It("should deny exec access for postgres pod", func() {
				result := client.CoreV1().RESTClient().Post().Namespace(postgresPod.Namespace).Resource("pods").Name(postgresPod.Name).SubResource("exec").Do(context.Background())
				gomega.Expect(result.Error()).To(gomega.MatchError(gomega.ContainSubstring("exec into postgres pods is forbidden")))
			})

			g.It("should deny exec access for system pod", func() {
				result := client.CoreV1().RESTClient().Post().Namespace(systemPod.Namespace).Resource("pods").Name(systemPod.Name).SubResource("exec").Do(context.Background())
				gomega.Expect(result.Error()).To(gomega.MatchError(gomega.ContainSubstring("write operations are forbidden")))
			})
		})
	})
})

// getAdminClient returns a client with the `zalando:administrator` group.
func getAdminClient(cluster *types.Cluster, awsAccountID string) (*kubernetes.Clientset, error) {
	return newClientWithRole(cluster, fmt.Sprintf("arn:aws:iam::%s:role/%s-e2e-eks-iam-test-administrator-role", awsAccountID, aws.ToString(cluster.Name)))
}

// getCollaboratorClient returns a client with the `zalando:collaborator` group.
func getCollaboratorClient(cluster *types.Cluster, awsAccountID string) (*kubernetes.Clientset, error) {
	return newClientWithRole(cluster, fmt.Sprintf("arn:aws:iam::%s:role/%s-e2e-eks-iam-test-collaborator-role", awsAccountID, aws.ToString(cluster.Name)))
}

// getEngineerClient returns a client with the `zalando:engineer` group.
func getEngineerClient(cluster *types.Cluster, awsAccountID string) (*kubernetes.Clientset, error) {
	return newClientWithRole(cluster, fmt.Sprintf("arn:aws:iam::%s:role/%s-e2e-eks-iam-test-engineer-role", awsAccountID, aws.ToString(cluster.Name)))
}

// getReadOnlyClient returns a client with the `zalando:readonly` group.
func getReadOnlyClient(cluster *types.Cluster, awsAccountID string) (*kubernetes.Clientset, error) {
	return newClientWithRole(cluster, fmt.Sprintf("arn:aws:iam::%s:role/%s-e2e-eks-iam-test-read-only-role", awsAccountID, aws.ToString(cluster.Name)))
}

// getPostgresAdministratorClient returns a client with the `zalando:postgres-admin` group.
func getPostgresAdministratorClient(cluster *types.Cluster, awsAccountID string) (*kubernetes.Clientset, error) {
	return newClientWithRole(cluster, fmt.Sprintf("arn:aws:iam::%s:role/%s-e2e-eks-iam-test-postgres-admin-role", awsAccountID, aws.ToString(cluster.Name)))
}

// newClientWithRole returns a new Kubernetes client with the specified IAM role and its associated AccessEntries.
func newClientWithRole(cluster *types.Cluster, assumeRole string) (*kubernetes.Clientset, error) {
	gen, err := token.NewGenerator(true, false)
	if err != nil {
		return nil, err
	}
	opts := &token.GetTokenOptions{
		ClusterID:     aws.ToString(cluster.Name),
		AssumeRoleARN: assumeRole,
	}
	tok, err := gen.GetWithOptions(opts)
	if err != nil {
		return nil, err
	}
	ca, err := base64.StdEncoding.DecodeString(aws.ToString(cluster.CertificateAuthority.Data))
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(
		&rest.Config{
			Host:        aws.ToString(cluster.Endpoint),
			BearerToken: tok.Token,
			TLSClientConfig: rest.TLSClientConfig{
				CAData: ca,
			},
		},
	)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

// getEKSCluster returns the EKS cluster where its Endpoint matches the given config's Host.
func getEKSCluster(ctx context.Context, awsConfig aws.Config, config *rest.Config) (*types.Cluster, error) {
	client := eks.NewFromConfig(awsConfig)

	listClusters, err := client.ListClusters(ctx, &eks.ListClustersInput{})
	if err != nil {
		return nil, err
	}

	for _, clusterName := range listClusters.Clusters {
		describeCluster, err := client.DescribeCluster(ctx, &eks.DescribeClusterInput{
			Name: aws.String(clusterName),
		})
		if err != nil {
			return nil, err
		}
		if aws.ToString(describeCluster.Cluster.Endpoint) == config.Host {
			return describeCluster.Cluster, nil
		}
	}

	return nil, fmt.Errorf("cluster not found: %s", config.Host)
}

// examplePod returns an example Pod with the specified namespace and labels.
func examplePod(namespace string, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-pod-",
			Namespace:    namespace,
			Labels:       labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "pause",
				Image: "container-registry.zalando.net/teapot/pause:3.7-master-21",
			}},
		},
	}
}

// createPod starts a Pod in the specified namespace and with the specific labels.
func createPod(ctx context.Context, client clientset.Interface, namespace string, labels map[string]string) (*corev1.Pod, error) {
	pod, err := client.CoreV1().Pods(namespace).Create(ctx, examplePod(namespace, labels), metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	if err := testutil.WaitForPodsWithLabelRunning(client, namespace, kubelabels.SelectorFromSet(kubelabels.Set(labels))); err != nil {
		return nil, err
	}

	return pod, nil
}

// createClusterRole creates a ClusterRole with the specified labels.
func createClusterRole(ctx context.Context, client clientset.Interface, labels map[string]string) (*rbacv1.ClusterRole, error) {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-cluster-role-",
			Labels:       labels,
		},
	}

	return client.RbacV1().ClusterRoles().Create(ctx, clusterRole, metav1.CreateOptions{})
}

// getAWSAccountID returns the current AWS account's ID.
func getAWSAccountID(ctx context.Context, awsConfig aws.Config) (string, error) {
	client := sts.NewFromConfig(awsConfig)

	callerIdentity, err := client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}

	return aws.ToString(callerIdentity.Account), nil
}
