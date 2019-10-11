package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/kubernetes/test/e2e/framework"
)

type expect struct {
	status int
	body   string
}

const (
	authzAPIVersion          = "authorization.k8s.io/v1beta1"
	authorizeMessageKind     = "SubjectAccessReview"
	systemMastersGroup       = "system:masters"
	operatorGroup            = "Operator"
	powerUserGroup           = "PowerUser"
	emergencyGroup           = "Emergency"
	manualGroup              = "Manual"
	controllerGroup          = "ControllerUser"
	readOnlyGroup            = "ReadOnly"
	portForwardPodNamePrefix = "port-forward-"
	systemNamespace          = "kube-system"
	accessReviewURL          = "/apis/authorization.k8s.io/v1beta1/subjectaccessreviews"
)

type authorizationResponseStatus struct {
	Allowed bool   `json:"allowed,omitempty"`
	Denied  bool   `json:"denied,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

type authorizationResp struct {
	apiHeader
	Status authorizationResponseStatus `json:"status"`
}

type apiHeader struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
}

var _ = framework.KubeDescribe("Authorization tests", func() {

	It("Should validate permissions in the cluster [Authorization] [RBAC] [Zalando]", func() {
		conf, _ := framework.LoadConfig()
		host := conf.Host
		client := http.DefaultClient
		makeReq := newReqBuilder(host+accessReviewURL, conf.BearerToken)

		for _, ti := range []struct {
			msg     string
			reqBody string
			expect  expect
		}{
			{
				msg: "kubelet authorized",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "teapot",
						"verb": "get",
						"group": "",
						"resource": "pods"
					},
					"user": "kubelet",
					"group": [
						"system:masters"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			}, {
				msg: "kube-system default account can update daemonset status",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "kube-system",
						"verb": "update",
						"group": "extensions",
						"resource": "daemonsets",
						"subresource": "status"
					},
					"user": "system:serviceaccount:kube-system:daemon-set-controller",
					"group": ["system:serviceaccounts:kube-system"]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			}, {
				msg: "kube-system default account can update daemonset finalizers",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "kube-system",
						"verb": "update",
						"group": "extensions",
						"resource": "daemonsets",
						"subresource": "finalizers"
					},
					"user": "system:serviceaccount:kube-system:daemon-set-controller",
					"group": ["system:serviceaccounts:kube-system"]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			}, {
				msg: "kube-system default account can list podtemplates",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "kube-system",
						"verb": "list",
						"resource": "podtemplates"
					},
					"user": "system:serviceaccount:kube-system:default",
					"group": ["system:serviceaccounts:kube-system"]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			}, {
				msg: "default account in default namespace can list statefulsets",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "default",
						"verb": "list",
						"resource": "statefulsets"
					},
					"user": "system:serviceaccount:default:default",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			}, {
				msg: "default account in non-default namespace can list statefulsets",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "non-default",
						"verb": "list",
						"resource": "statefulsets"
					},
					"user": "system:serviceaccount:non-default:default",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			}, {
				msg: "User in admin group can patch daemonsets",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "kube-system",
						"name": "prometheus-node-exporter",
						"verb": "patch",
						"group": "extensions",
						"resource": "daemonsets"
					},
					"user": "sszuecs",
					"group": [
						"ReadOnly",
						"system:masters",
						"system:authenticated"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			}, {
				msg: "non-authorized group",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "teapot",
						"verb": "get",
						"group": "",
						"resource": "pods"
					},
					"user": "rdifazio",
					"group": [
						"FooBar"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": false,
						"reason": "unauthorized access rdifazio/[FooBar]"
					}
				}}`,
				},
			}, {
				msg: "resource list authorized with ReadOnly group",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "teapot",
						"verb": "list",
						"group": "",
						"resource": "pods"
					},
					"user": "rdifazio",
					"group": [
						"ReadOnly"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			}, {
				msg: "access to non-resource path with ReadOnly group",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"nonResourceAttributes": {
						"path": "/apis",
						"verb": "get"
					},
					"user": "mlarsen",
					"group": [
						"ReadOnly"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			}, {
				msg: "access to use PodSecurityPolicy for ReadOnly should not be allowed",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"name": "privileged",
						"namespace": "",
						"verb": "use",
						"group": "extensions",
						"resource": "podsecuritypolicies"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, readOnlyGroup),
				expect: expect{
					status: http.StatusCreated,
					body: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": false,
						"reason":"unauthorized access sszuecs/[%s]"
					}
				}}`, readOnlyGroup),
				},
			}, {
				msg: "ReadOnly role should not give port-forward access to the 'port-forward-' pod in default namespace",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
						"resourceAttributes": {
							"name": "port-forward-abc",
							"namespace": "default",
							"verb": "create",
							"group": "",
							"resource": "pods",
							"subresource": "portforward"
						},
						"user": "read-only-user",
						"group": [
							"%s"
						]
					}
				}`, readOnlyGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": false
					}
				}}`,
				},
			}, {
				msg: "ReadOnly role should give read access to nodes",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
						"resourceAttributes": {
							"namespace": "",
							"verb": "get",
							"group": "",
							"resource": "nodes"
						},
						"user": "read-only-user",
						"group": [
							"%s"
						]
					}
				}`, readOnlyGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
						"apiVersion": "authorization.k8s.io/v1beta1",
						"kind": "SubjectAccessReview",
						"status": {
							"allowed": true
						}
					}`,
				},
			},

			//- poweruser can use restricted psp
			{
				msg: "access to use restricted PodSecurityPolicy for PowerUser should be allowed",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
						"resourceAttributes": {
							"name": "restricted",
							"namespace": "",
							"verb": "use",
							"group": "extensions",
							"resource": "podsecuritypolicies"
						},
						"user": "sszuecs",
						"group": [
							"%s"
						]
					}
				}`, powerUserGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
						"apiVersion": "authorization.k8s.io/v1beta1",
						"kind": "SubjectAccessReview",
						"status": {
							"allowed": true
						}
				}}`,
				},
			},

			//- emergency can use restricted psp
			{
				msg: "access to use restricted PodSecurityPolicy for Emergency should be allowed",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"name": "restricted",
						"namespace": "",
						"verb": "use",
						"group": "extensions",
						"resource": "podsecuritypolicies"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, emergencyGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
						"apiVersion": "authorization.k8s.io/v1beta1",
						"kind": "SubjectAccessReview",
						"status": {
							"allowed": true
						}
				}}`,
				},
			},

			//- Manual role can use restricted psp
			{
				msg: "access to use restricted PodSecurityPolicy for Manual role should be allowed",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"name": "restricted",
						"namespace": "",
						"verb": "use",
						"group": "extensions",
						"resource": "podsecuritypolicies"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, manualGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
						"apiVersion": "authorization.k8s.io/v1beta1",
						"kind": "SubjectAccessReview",
						"status": {
							"allowed": true
						}
				}}`,
				},
			},

			////- poweruser can not use privileged PSP
			//// TODO: disable privileged PSP access for PowerUsers.
			//{
			//	msg: "access to use privileged PodSecurityPolicy for PowerUser should not be allowed",
			//	reqBody: fmt.Sprintf(`{
			//		"apiVersion": "authorization.k8s.io/v1beta1",
			//		"kind": "SubjectAccessReview",
			//		"spec": {
			//		"resourceAttributes": {
			//			"name": "privileged",
			//			"namespace": "",
			//			"verb": "use",
			//			"group": "extensions",
			//			"resource": "podsecuritypolicies"
			//		},
			//		"user": "sszuecs",
			//		"group": [
			//			"%s"
			//		]
			//		}
			//	}`, powerUserGroup),
			//	expect: expect{
			//		status: http.StatusCreated,
			//		body: fmt.Sprintf(`{
			//		"apiVersion": "authorization.k8s.io/v1beta1",
			//		"kind": "SubjectAccessReview",
			//		"status": {
			//			"allowed": false,
			//			"reason":"unauthorized access sszuecs/[%s]"
			//		}
			//	}}`, powerUserGroup),
			//	},
			//},

			//- poweruser has read access to kube system
			{
				msg: "PowerUser has read access (pods) to kube-system",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "kube-system",
						"verb": "get",
						"group": "",
						"resource": "pods"
					},
					"user": "rdifazio",
					"group": [
						"PowerUser"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},

			//- poweruser has no access to kube-system secrets
			{
				msg: "PowerUser has no read access to kube-system secrets",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "kube-system",
						"verb": "get",
						"group": "",
						"resource": "secrets"
					},
					"user": "sszuecs",
					"group": [
						"PowerUser"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": false,
						"denied": true,
						"reason":"unauthorized access sszuecs/[PowerUser]"
					}
				}}`,
				},
			},

			//- poweruser can read secrets from non kube-system namespaces
			{
				msg: "PowerUser has read access to non kube-system secrets",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "teapot",
						"verb": "get",
						"group": "",
						"resource": "secrets"
					},
					"user": "sszuecs",
					"group": [
						"PowerUser"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},

			//- poweruser has write access to non kube-system namespaces
			{
				msg: "PowerUser has write access to non kube-system secrets",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "teapot",
						"verb": "create",
						"group": "",
						"resource": "secrets"
					},
					"user": "sszuecs",
					"group": [
						"PowerUser"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},

			//- TODO poweruser has exec right
			//- CHECK poweruser has proxy right
			{
				msg: "PowerUser has proxy right",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "teapot",
						"verb": "create",
						"group": "",
						"resource": "pods/proxy"
					},
					"user": "sszuecs",
					"group": [
						"PowerUser"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},
			//- CHECK poweruser can not create daemonsets
			{
				msg: "PowerUser has no create access to daemonsets",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "teapot",
						"verb": "create",
						"group": "",
						"resource": "daemonsets"
					},
					"user": "sszuecs",
					"group": [
						"PowerUser"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": false
					}
				}}`,
				},
			},
			//- CHECK poweruser can not update daemonsets
			{
				msg: "PowerUser has no update access to daemonsets",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "teapot",
						"verb": "update",
						"group": "apps",
						"resource": "daemonsets"
					},
					"user": "sszuecs",
					"group": [
						"PowerUser"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": false
					}
				}}`,
				},
			},
			//- CHECK poweruser can not delete daemonsets
			{
				msg: "PowerUser has no delete access to daemonsets",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "teapot",
						"verb": "delete",
						"group": "apps",
						"resource": "daemonsets"
					},
					"user": "sszuecs",
					"group": [
						"PowerUser"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": false
					}
				}}`,
				},
			},
			//- CHECK poweruser can not patch daemonsets
			{
				msg: "PowerUser has no patch access to daemonsets",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "teapot",
						"verb": "patch",
						"group": "apps",
						"resource": "daemonsets"
					},
					"user": "sszuecs",
					"group": [
						"PowerUser"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": false
					}
				}}`,
				},
			},

			//- operator is allowed to use privileged PSP
			// Namespace is currently always empty string, because in Kubernetes PSPs are not namespaced, yet.
			// Check Kubernetes >= 1.7 if they namespaced it https://github.com/kubernetes/kubernetes/pull/42360
			{
				msg: "operator is allowed to use privileged PodSecurityPolicy (for own namespace)",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"name": "privileged",
						"namespace": "",
						"verb": "use",
						"group": "extensions",
						"resource": "podsecuritypolicies"
					},
					"user": "system:serviceaccount:teapot:operator",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},

			//- operator has read access to own namespace
			{
				msg: "operator has read access to own namespace",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "teapot",
						"verb": "get",
						"group": "",
						"resource": "pods"
					},
					"user": "system:serviceaccount:teapot:operator",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},

			//- operator has write access to own namespace
			{
				msg: "operator has write access to own namespace",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "teapot",
						"verb": "create",
						"group": "",
						"resource": "pods"
					},
					"user": "system:serviceaccount:teapot:operator",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},
			//- operator has read access to other namespaces
			{
				msg: "operator has read access to other namespace",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "coffeepot",
						"verb": "get",
						"group": "",
						"resource": "pods"
					},
					"user": "system:serviceaccount:teapot:operator",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},

			//- operator has no write access to other namespaces (not own)
			{
				msg: "operator has no write access to other namespace",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "coffeepot",
						"verb": "create",
						"group": "",
						"resource": "pods"
					},
					"user": "system:serviceaccount:teapot:operator",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": false,
						"reason": "unauthorized access system:serviceaccount:teapot:operator/[]"
					}
				}}`,
				},
			},

			//- operator has read access to secrets in own namespace
			{
				msg: "operator has read access to secrets in own namespace",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "teapot",
						"verb": "get",
						"group": "",
						"resource": "secrets"
					},
					"user": "system:serviceaccount:teapot:operator",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},

			//- operator is not allowed to read secrets in other namespaces
			{
				msg: "operator is not allowed to read secrets in other namespaces",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "coffeepot",
						"verb": "get",
						"group": "",
						"resource": "secrets"
					},
					"user": "system:serviceaccount:teapot:operator",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": false,
						"reason": "unauthorized access system:serviceaccount:teapot:operator/[]"
					}
				}}`,
				},
			},

			//- operator has read access to custom resource definitions (CRD) in all namespaces
			{
				msg: "operator has read access to custom resource definitions (CRD) in all namespacese",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "get",
						"group": "apiextensions.k8s.io",
						"resource": "customresourcedefinitions"
					},
					"user": "system:serviceaccount:teapot:operator",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},
			//- operator has write access to custom resource definitions (CRD) in all namespaces
			{
				msg: "operator has read access to custom resource definitions (CRD) in all namespacese",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "create",
						"group": "apiextensions.k8s.io",
						"resource": "customresourcedefinitions"
					},
					"user": "system:serviceaccount:teapot:operator",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},
			//- operator has write access to storageclasses in all namespaces
			{
				msg: "operator has write access to storageclasses in all namespaces",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"verb": "create",
						"group": "storage.k8s.io",
						"resource": "storageclasses"
					},
					"user": "system:serviceaccount:teapot:operator",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},
			//- operator has read access to storageclasses in all namespaces
			{
				msg: "operator has read access to storageclasses in all namespaces",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"verb": "get",
						"group": "storage.k8s.io",
						"resource": "storageclasses"
					},
					"user": "system:serviceaccount:teapot:operator",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},
			//- operator has read access to nodes in global namespace
			{
				msg: "operator has read access to nodes in global namespace",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"verb": "get",
						"group": "",
						"resource": "nodes"
					},
					"user": "system:serviceaccount:teapot:operator",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},
			//- operator has write access to nodes in global namespace
			{
				msg: "operator has write access to nodes in global namespace",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"verb": "create",
						"group": "",
						"resource": "nodes"
					},
					"user": "system:serviceaccount:teapot:operator",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},
			//- readonly is not allowed to read secrets all namespaces
			{
				msg: "readonly is not allowed to read secrets all namespaces",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "coffeepot",
						"verb": "get",
						"group": "",
						"resource": "secrets"},
					"user": "mkerk",
					"group": ["ReadOnly"]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": false,
						"reason": "unauthorized access mkerk/[ReadOnly]"
					}
				}}`,
				},
			},

			//- readonly is not allowed to use proxy
			{
				msg: "readonly is not allowed to use proxy",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "coffeepot",
						"verb": "proxy",
						"group": "",
						"resource": "services"
					},
					"user": "mkerk",
					"group": ["ReadOnly"]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": false,
						"reason": "unauthorized access mkerk/[ReadOnly]"
					}
				}}`,
				},
			},

			//- TODO: readonly is not allowed to use exec
			//- readonly has no write access to any resource
			{
				msg: "readonly has no write access to any resource",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "coffeepot",
						"verb": "create",
						"group": "",
						"resource": "secrets"
					},
					"user": "mkerk",
					"group": ["ReadOnly"]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": false,
						"reason": "unauthorized access mkerk/[ReadOnly]"
					}
				}}`,
				},
			},

			//- ReadOnly role cannot delete resources
			{
				msg: "ReadOnly role cannot delete resources",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "default",
						"verb": "delete",
						"group": "",
						"resource": "pods"
					},
					"user": "rdifazio",
					"group": [
						"ReadOnly"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": false,
						"reason":"unauthorized access rdifazio/[ReadOnly]"
					}
				}}`,
				},
			},

			//- Manual role can delete resources in all namespaces but kube-system
			{
				msg: "Manual role can delete resources in all namespaces except kube-system",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "default",
						"verb": "delete",
						"group": "",
						"resource": "pods"
					},
					"user": "rdifazio",
					"group": [
						"ReadOnly",
						"Manual"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},

			//- Manual role cannot delete resources in kube-sytem namespace
			{
				msg: "Manual role cannot delete resources in kube-sytem namespace",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "kube-system",
						"verb": "delete",
						"group": "",
						"resource": "pods"
					},
					"user": "rdifazio",
					"group": [
						"ReadOnly",
						"Manual"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": false,
						"denied": true,
						"reason":"unauthorized access rdifazio/[ReadOnly Manual]"
					}
				}}`,
				},
			},

			//- Manual role can delete namespaces
			{
				msg: "Manual role can delete namespaces",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "delete",
						"group": "",
						"resource": "namespaces"
					},
					"user": "rdifazio",
					"group": [
						"ReadOnly",
						"Manual"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},

			//- Manual role can't delete kube-system namespace
			{
				msg: "Manual role can't delete kube-system namespace",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"verb": "delete",
						"group": "",
						"resource": "namespaces",
						"name": "kube-system"
					},
					"user": "rdifazio",
					"group": [
						"ReadOnly",
						"Manual"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": false,
						"denied": true,
						"reason":"unauthorized access rdifazio/[ReadOnly Manual]"
					}
				}}`,
				},
			},

			//- Manual role can create resources
			{
				msg: "Manual role can create resources",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "default",
						"verb": "create",
						"group": "",
						"resource": "pods"
					},
					"user": "rdifazio",
					"group": [
						"ReadOnly",
						"Manual"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},

			//- Manual role doesn't affect funtionality of other roles.
			{
				msg: "Manual role doesn't affect funtionality of other roles.",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "default",
						"verb": "get",
						"group": "",
						"resource": "pods"
					},
					"user": "rdifazio",
					"group": [
						"ReadOnly",
						"Manual"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},

			//- administrator can use restricted PSP
			{
				msg: "access to use PodSecurityPolicy for Administrator (system:masters) should be allowed",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"name": "restricted",
						"namespace": "",
						"verb": "use",
						"group": "extensions",
						"resource": "podsecuritypolicies"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, systemMastersGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},

			//- administrator can use privileged PSP
			{
				msg: "access to use PodSecurityPolicy for Administrator (system:masters) should be allowed",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"name": "privileged",
						"namespace": "",
						"verb": "use",
						"group": "extensions",
						"resource": "podsecuritypolicies"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, systemMastersGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},

			//- system:masters can use privileged PSP
			{
				msg: "access to use PodSecurityPolicy for system:masters should be allowed",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"name": "privileged",
						"namespace": "",
						"verb": "use",
						"group": "extensions",
						"resource": "podsecuritypolicies"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, systemMastersGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},

			//- Controller manager can list podsecurity policies
			{
				msg: "controller manager can list podsecurity policies",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "list",
						"group": "extensions",
						"resource": "podsecuritypolicies"
					},
					"user": "system:kube-controller-manager"
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},

			//- administrator has read access to kube system
			{
				msg: "Administrator (system:masters) has read access (pods) to kube-system",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "kube-system",
						"verb": "get",
						"group": "",
						"resource": "pods"
					},
					"user": "rdifazio",
					"group": [
						"system:masters"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},
			//- administrator has write access to kube system
			{
				msg: "Administrator (system:masters) has write access (pods) to kube-system",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "kube-system",
						"verb": "create",
						"group": "",
						"resource": "pods"
					},
					"user": "rdifazio",
					"group": [
						"system:masters"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},
			//- administrator can read secrets from kube-system namespaces
			{
				msg: "Administrator (system:masters) can read secrets from kube-system namespaces",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "kube-system",
						"verb": "get",
						"group": "",
						"resource": "secrets"
					},
					"user": "rdifazio",
					"group": [
						"system:masters"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},
			//- administrator can read secrets from non kube-system namespaces
			{
				msg: "Administrator (system:masters) can read secrets from non kube-system namespaces",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "teapot",
						"verb": "get",
						"group": "",
						"resource": "secrets"
					},
					"user": "rdifazio",
					"group": [
						"system:masters"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},
			//- administrator has write access to non kube-system namespaces
			{
				msg: "Administrator (system:masters) has write access to non kube-system namespaces",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "teapot",
						"verb": "create",
						"group": "",
						"resource": "pods"
					},
					"user": "rdifazio",
					"group": [
						"system:masters"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},
			//- TODO administrator has exec right

			//- administrator has proxy right
			{
				msg: "Administrator (system:masters) has proxy right",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "teapot",
						"verb": "proxy",
						"group": ""
					},
					"user": "sszuecs",
					"group": [
						"system:masters"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},
			//- administrator can write daemonsets
			{
				msg: "Administrator (system:masters) can write daemonsets",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "teapot",
						"verb": "create",
						"group": "apps",
						"resource": "daemonsets"
					},
					"user": "sszuecs",
					"group": [
						"system:masters"
					]
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},
			{
				msg: "cdp service account can create namespaces",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "create",
						"group": "",
						"resource": "namespaces"
					},
					"user": "system:serviceaccount:default:cdp",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},
			{
				msg: "cdp service account can't escalate permissions",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "escalate",
						"group": "rbac.authorization.k8s.io",
						"resource": "clusterroles"
					},
					"user": "system:serviceaccount:default:cdp",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": false
					}
				}}`,
				},
			},
			{
				msg: "PowerUsers can't escalate permissions",
				reqBody: `{
				"apiVersion": "authorization.k8s.io/v1beta1",
				"kind": "SubjectAccessReview",
				"spec": {
				"resourceAttributes": {
					"namespace": "",
					"verb": "escalate",
					"group": "rbac.authorization.k8s.io",
					"resource": "clusterroles"
				},
				"user": "mlarsen",
				"group": ["PowerUser"]
				}
			}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
				"apiVersion": "authorization.k8s.io/v1beta1",
				"kind": "SubjectAccessReview",
				"status": {
					"allow": false
				}
			}}`,
				},
			},
			{
				msg: "operator service account cannot create namespaces",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "create",
						"group": "",
						"resource": "namespaces"
					},
					"user": "system:serviceaccount:default:operator",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": false,
						"reason": "unauthorized access system:serviceaccount:default:operator/[]"
					}
				}}`,
				},
			},
			{
				msg: "controller manager service account can create pods",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "kube-system",
						"verb": "create",
						"group": "",
						"resource": "pods"
					},
					"user": "system:serviceaccount:kube-system:daemon-set-controller",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true
					}
				}}`,
				},
			},
			{
				msg: "operator service account can access persistent volumes in other namespaces",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "get",
						"group": "",
						"resource": "persistentvolumes"
					},
					"user": "system:serviceaccount:default:operator",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true,
						"reason": ""
					}
				}}`,
				},
			},
			{
				msg: "persistent volume binder service account can update kube system persistentVolumeClaims",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "kube-system",
						"verb": "update",
						"group": "",
						"resource": "persistentvolumeclaims"
					},
					"user": "system:serviceaccount:kube-system:persistent-volume-binder",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true,
						"reason": ""
					}
				}}`,
				},
			},
			{
				msg: "persistent volume binder service account can create kube system persistentVolumes",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "kube-system",
						"verb": "create",
						"group": "",
						"resource": "persistentvolumes"
					},
					"user": "system:serviceaccount:kube-system:persistent-volume-binder",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true,
						"reason": ""
					}
				}}`,
				},
			},
			{
				msg: "horizontal pod autoscaler service account can update kube system autoscalers",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "kube-system",
						"verb": "update",
						"group": "*",
						"resource": "*/scale"
					},
					"user": "system:serviceaccount:kube-system:horizontal-pod-autoscaler",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true,
						"reason": ""
					}
				}}`,
				},
			},
			{
				msg: "horizontal pod autoscaler service account can update any autoscaler",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "*",
						"verb": "update",
						"group": "*",
						"resource": "*/scale"
					},
					"user": "system:serviceaccount:kube-system:horizontal-pod-autoscaler",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true,
						"reason": ""
					}
				}}`,
				},
			},
			{
				msg: "aws-cloud-provider service account can access patch nodes",
				reqBody: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"verb": "patch",
						"group": "",
						"resource": "nodes"
					},
					"user": "system:serviceaccount:kube-system:aws-cloud-provider",
					"group": []
					}
				}`,
				expect: expect{
					status: http.StatusCreated,
					body: `{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"status": {
						"allowed": true,
						"reason": ""
					}
				}}`,
				},
			},
			{
				msg: "emergency user should not have update access to node resources.",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "update",
						"group": "",
						"resource": "nodes"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, emergencyGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
						"apiVersion": "authorization.k8s.io/v1beta1",
						"kind": "SubjectAccessReview",
						"status": {
							"allowed": false
						}
				}}`,
				},
			},
			{
				msg: "manual user should not have non update to node resources.",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "update",
						"group": "",
						"resource": "nodes"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, manualGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
						"apiVersion": "authorization.k8s.io/v1beta1",
						"kind": "SubjectAccessReview",
						"status": {
							"allowed": false
						}
				}}`,
				},
			},
			{
				msg: "power user should not have update access to node resources.",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "update",
						"group": "",
						"resource": "nodes"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, powerUserGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
						"apiVersion": "authorization.k8s.io/v1beta1",
						"kind": "SubjectAccessReview",
						"status": {
							"allowed": false
						}
				}}`,
				},
			},
			{
				msg: "emergency user should not have create access to node resources.",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "create",
						"group": "",
						"resource": "nodes"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, emergencyGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
						"apiVersion": "authorization.k8s.io/v1beta1",
						"kind": "SubjectAccessReview",
						"status": {
							"allowed": false
						}
				}}`,
				},
			},
			{
				msg: "manual user should not have create access to node resources.",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "create",
						"group": "",
						"resource": "nodes"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, manualGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
						"apiVersion": "authorization.k8s.io/v1beta1",
						"kind": "SubjectAccessReview",
						"status": {
							"allowed": false
						}
				}}`,
				},
			},
			{
				msg: "power user should not have create access to node resources.",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "create",
						"group": "",
						"resource": "nodes"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, powerUserGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
						"apiVersion": "authorization.k8s.io/v1beta1",
						"kind": "SubjectAccessReview",
						"status": {
							"allowed": false
						}
				}}`,
				},
			},
			{
				msg: "emergency user should not have patch access to node resources.",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "patch",
						"group": "",
						"resource": "nodes"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, emergencyGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
						"apiVersion": "authorization.k8s.io/v1beta1",
						"kind": "SubjectAccessReview",
						"status": {
							"allowed": false
						}
				}}`,
				},
			},
			{
				msg: "manual user should not have patch access to node resources.",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "patch",
						"group": "",
						"resource": "nodes"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, manualGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
						"apiVersion": "authorization.k8s.io/v1beta1",
						"kind": "SubjectAccessReview",
						"status": {
							"allowed": false
						}
				}}`,
				},
			},
			{
				msg: "power user should not have patch access to node resources.",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "patch",
						"group": "",
						"resource": "nodes"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, powerUserGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
						"apiVersion": "authorization.k8s.io/v1beta1",
						"kind": "SubjectAccessReview",
						"status": {
							"allowed": false
						}
				}}`,
				},
			},
			{
				msg: "emergency user should not have delete access to node resources.",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "delete",
						"group": "",
						"resource": "nodes"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, emergencyGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
						"apiVersion": "authorization.k8s.io/v1beta1",
						"kind": "SubjectAccessReview",
						"status": {
							"allowed": false
						}
				}}`,
				},
			},
			{
				msg: "manual user should not have delete access to node resources.",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "delete",
						"group": "",
						"resource": "nodes"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, manualGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
						"apiVersion": "authorization.k8s.io/v1beta1",
						"kind": "SubjectAccessReview",
						"status": {
							"allowed": false
						}
				}}`,
				},
			},
			{
				msg: "power user should not have delete access to node resources.",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "delete",
						"group": "",
						"resource": "nodes"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, powerUserGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
						"apiVersion": "authorization.k8s.io/v1beta1",
						"kind": "SubjectAccessReview",
						"status": {
							"allowed": false
						}
				}}`,
				},
			},
			{
				msg: "power user should be allowed list access to node resources.",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "list",
						"group": "",
						"resource": "nodes"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, powerUserGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
						"apiVersion": "authorization.k8s.io/v1beta1",
						"kind": "SubjectAccessReview",
						"status": {
							"allowed": true
						}
				}}`,
				},
			},
			{
				msg: "emergency user should be allowed list access to node resources.",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "list",
						"group": "",
						"resource": "nodes"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, emergencyGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
						"apiVersion": "authorization.k8s.io/v1beta1",
						"kind": "SubjectAccessReview",
						"status": {
							"allowed": true
						}
				}}`,
				},
			},
			{
				msg: "manual user should be allowed list access to node resources.",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "list",
						"group": "",
						"resource": "nodes"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, manualGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
						"apiVersion": "authorization.k8s.io/v1beta1",
						"kind": "SubjectAccessReview",
						"status": {
							"allowed": true
						}
				}}`,
				},
			},
			{
				msg: "power user should be allowed read access to node resources.",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "get",
						"group": "",
						"resource": "nodes"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, powerUserGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
						"apiVersion": "authorization.k8s.io/v1beta1",
						"kind": "SubjectAccessReview",
						"status": {
							"allowed": true
						}
				}}`,
				},
			},
			{
				msg: "emergency user should be allowed read access to node resources.",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "get",
						"group": "",
						"resource": "nodes"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, emergencyGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
						"apiVersion": "authorization.k8s.io/v1beta1",
						"kind": "SubjectAccessReview",
						"status": {
							"allowed": true
						}
				}}`,
				},
			},
			{
				msg: "manual user should be allowed read access to node resources.",
				reqBody: fmt.Sprintf(`{
					"apiVersion": "authorization.k8s.io/v1beta1",
					"kind": "SubjectAccessReview",
					"spec": {
					"resourceAttributes": {
						"namespace": "",
						"verb": "get",
						"group": "",
						"resource": "nodes"
					},
					"user": "sszuecs",
					"group": [
						"%s"
					]
					}
				}`, manualGroup),
				expect: expect{
					status: http.StatusCreated,
					body: `{
						"apiVersion": "authorization.k8s.io/v1beta1",
						"kind": "SubjectAccessReview",
						"status": {
							"allowed": true
						}
				}}`,
				},
			},
		} {

			By(ti.msg)

			req, err := makeReq(ti.reqBody)
			Expect(err).NotTo(HaveOccurred())

			resp, err := client.Do(req)
			Expect(err).NotTo(HaveOccurred())

			body, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())

			if resp.StatusCode != ti.expect.status {
				framework.Failf("%s: invalid status code received. expected %d, got %d\n%s", ti.msg, ti.expect.status, resp.StatusCode, string(body))
				return
			}

			var authzResp authorizationResp
			if err := json.Unmarshal(body, &authzResp); err != nil && err != io.EOF {
				framework.Failf(ti.msg, err)
				return
			}

			var expectedRspDoc authorizationResp
			dec := json.NewDecoder(bytes.NewBufferString(ti.expect.body))
			if err := dec.Decode(&expectedRspDoc); err != nil && err != io.EOF {
				framework.Failf(ti.msg, err)
				return
			}

			if authzResp.Status.Allowed != expectedRspDoc.Status.Allowed || authzResp.Status.Denied != expectedRspDoc.Status.Denied {
				framework.Failf("unexpected response. expected %v, got %v", expectedRspDoc, authzResp)
			}
		}
	})
})

func newReqBuilder(url, token string) func(string) (*http.Request, error) {
	return func(body string) (*http.Request, error) {
		req, err := http.NewRequest("POST", url, bytes.NewBufferString(body))
		if err != nil {
			return nil, err
		}

		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		return req, err
	}
}
