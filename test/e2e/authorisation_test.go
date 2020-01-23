package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	authzAPIVersion      = "authorization.k8s.io/v1beta1"
	authorizeMessageKind = "SubjectAccessReview"
	systemMastersGroup   = "system:masters"
	powerUserGroup       = "PowerUser"
	emergencyGroup       = "Emergency"
	manualGroup          = "Manual"
	readOnlyGroup        = "ReadOnly"
	systemNamespace      = "kube-system"
	accessReviewURL      = "/apis/authorization.k8s.io/v1beta1/subjectaccessreviews"
)

type apiHeader struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
}

type resourceAttributes struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Verb        string `json:"verb"`
	Group       string `json:"group"`
	Resource    string `json:"resource"`
	Subresource string `json:"subresource"`
	Path        string `json:"path"`
}

type nonResourceAttributes struct {
	Verb string `json:"verb"`
	Path string `json:"path"`
}

type requestSpec struct {
	ResourceAttributes    *resourceAttributes    `json:"resourceAttributes,omitempty"`
	NonResourceAttributes *nonResourceAttributes `json:"nonResourceAttributes,omitempty"`
	User                  string                 `json:"user"`
	Group                 []string               `json:"group"`
}

type requestData struct {
	apiHeader
	Spec requestSpec `json:"spec"`
}

type authorizationResponseStatus struct {
	Allowed bool   `json:"allowed,omitempty"`
	Denied  bool   `json:"denied,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

type authorizationResp struct {
	apiHeader
	Status authorizationResponseStatus `json:"status"`
}

type response struct {
	status          int
	allowed, denied bool
	reason          []string
}

func req() requestData {
	return requestData{
		apiHeader: apiHeader{
			APIVersion: "authorization.k8s.io/v1beta1",
			Kind:       "SubjectAccessReview",
		},
		Spec: requestSpec{
			ResourceAttributes: &resourceAttributes{
				Verb: "get",
			},
		},
	}
}

func (r requestData) ns(ns string) requestData {
	r.Spec.ResourceAttributes.Namespace = ns
	return r
}

func (r requestData) name(n string) requestData {
	r.Spec.ResourceAttributes.Name = n
	return r
}

func (r requestData) verb(v string) requestData {
	r.Spec.ResourceAttributes.Verb = v
	return r
}

func (r requestData) resGroup(g string) requestData {
	r.Spec.ResourceAttributes.Group = g
	return r
}

func (r requestData) res(res string) requestData {
	r.Spec.ResourceAttributes.Resource = res
	return r
}

func (r requestData) subres(subres string) requestData {
	r.Spec.ResourceAttributes.Subresource = subres
	return r
}

func (r requestData) path(p string) requestData {
	r.Spec.ResourceAttributes.Path = p
	return r
}

func (r requestData) nonResVerb(v string) requestData {
	if r.Spec.NonResourceAttributes == nil {
		r.Spec.ResourceAttributes = nil
		r.Spec.NonResourceAttributes = &nonResourceAttributes{}
	}

	r.Spec.NonResourceAttributes.Verb = v
	return r
}

func (r requestData) nonResPath(p string) requestData {
	if r.Spec.NonResourceAttributes == nil {
		r.Spec.ResourceAttributes = nil
		r.Spec.NonResourceAttributes = &nonResourceAttributes{}
	}

	r.Spec.NonResourceAttributes.Path = p
	return r
}

func (r requestData) user(u string) requestData {
	r.Spec.User = u
	return r
}

func (r requestData) groups(g ...string) requestData {
	r.Spec.Group = g
	return r
}

func bindReason(rsp response) func(...string) response {
	return func(reason ...string) response {
		rsp.reason = reason
		return rsp
	}
}

func expect(status int, allowed, denied bool) response {
	return response{
		status:  status,
		allowed: allowed,
		denied:  denied,
	}
}

var (
	undecided       = expect(http.StatusCreated, false, false)
	allowed         = expect(http.StatusCreated, true, false)
	denied          = expect(http.StatusCreated, false, true)
	undecidedReason = bindReason(undecided)
	deniedReason    = bindReason(denied)
)

func newReqBuilder(url, token string) func(requestData) (*http.Request, error) {
	return func(body requestData) (*http.Request, error) {
		j, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(j))
		if err != nil {
			return nil, err
		}

		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		return req, err
	}
}

var _ = framework.KubeDescribe("Authorization tests", func() {

	It("Should validate permissions in the cluster [Authorization] [RBAC] [Zalando]", func() {
		conf, _ := framework.LoadConfig()
		host := conf.Host
		client := http.DefaultClient
		makeReq := newReqBuilder(host+accessReviewURL, conf.BearerToken)

		for _, ti := range []struct {
			msg    string
			req    requestData
			expect response
		}{
			{
				msg:    "kubelet authorized",
				req:    req().ns("teapot").verb("get").res("pods").user("kubelet").groups("system:masters"),
				expect: allowed,
			},

			{
				msg: "kube-system daemonset-controller service account can update daemonset status",
				req: req().ns("kube-system").verb("update").resGroup("extensions").res("daemonsets").subres("status").
					user("system:serviceaccount:kube-system:daemon-set-controller").
					groups("system:serviceaccounts:kube-system"),
				expect: allowed,
			},

			{
				msg: "kube-system default account can update daemonset finalizers",
				req: req().ns("kube-system").verb("update").resGroup("extensions").res("daemonsets").subres("finalizers").
					user("system:serviceaccount:kube-system:daemon-set-controller").
					groups("system:serviceaccounts:kube-system"),
				expect: allowed,
			},

			{
				msg:    "default account in default namespace can not list statefulsets",
				req:    req().verb("list").res("statefulsets").user("system:serviceaccount:default:default"),
				expect: undecided,
			},

			{
				msg:    "default account in non-default namespace can not list statefulsets",
				req:    req().ns("non-default").verb("list").res("statefulsets").user("system:serviceaccount:non-default:default"),
				expect: undecided,
			},

			{
				msg: "User in admin group can patch daemonsets",
				req: req().ns("kube-system").name("prometheus-node-exporter").verb("patch").resGroup("extensions").res("daemonsets").
					user("sszuecs").groups("ReadOnly", "system:masters", "system:authenticated"),
				expect: allowed,
			},

			{
				msg:    "non-authorized group",
				req:    req().ns("teapot").verb("get").res("pods").user("rdifazio").groups("FooBar"),
				expect: undecidedReason("access undecided rdifazio/[FooBar]"),
			},

			{
				msg:    "resource list authorized with ReadOnly group",
				req:    req().ns("teapot").verb("list").res("pods").user("rdifazio").groups("ReadOnly"),
				expect: allowed,
			},

			{
				msg:    "access to non-resource path with ReadOnly group",
				req:    req().nonResVerb("get").nonResPath("/apis").user("mlarsern").groups("ReadOnly"),
				expect: allowed,
			},

			{
				msg: "access to use PodSecurityPolicy for ReadOnly should not be allowed",
				req: req().name("privileged").verb("user").resGroup("extensions").res("podsecuritypolicies").
					user("sszuecs").groups(readOnlyGroup),
				expect: undecidedReason("access undecided sszuecs", readOnlyGroup),
			},

			{
				msg: "ReadOnly role should not give port-forward access to the 'port-forward-' pod in default namespace",
				req: req().name("port-forward-abc").verb("create").res("pods").subres("portforward").
					user("read-only-user").groups(readOnlyGroup),
				expect: undecided,
			},

			{
				msg:    "ReadOnly role should give read access to nodes",
				req:    req().verb("get").res("nodes").user("read-only-user").groups(readOnlyGroup),
				expect: allowed,
			},

			//- poweruser can use restricted psp
			{
				msg: "access to use restricted PodSecurityPolicy for PowerUser should be allowed",
				req: req().name("restricted").verb("use").resGroup("extensions").res("podsecuritypolicies").
					user("sszuecs").groups(powerUserGroup),
				expect: allowed,
			},

			//- emergency can use restricted psp
			{
				msg: "access to use restricted PodSecurityPolicy for Emergency should be allowed",
				req: req().name("restricted").verb("use").resGroup("extensions").res("podsecuritypolicies").
					user("sszuecs").groups(emergencyGroup),
				expect: allowed,
			},

			//- Manual role can use restricted psp
			{
				msg: "access to use restricted PodSecurityPolicy for Manual role should be allowed",
				req: req().name("restricted").verb("use").resGroup("extensions").res("podsecuritypolicies").
					user("sszuecs").groups(manualGroup),
				expect: allowed,
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
			//	expect: undecidedReason("unauthorized access sszuecs", powerUserGroup),
			//},

			//- poweruser has read access to kube system
			{
				msg:    "PowerUser has read access (pods) to kube-system",
				req:    req().ns("kube-system").verb("get").res("pods").user("rdifazio").groups("PowerUser"),
				expect: allowed,
			},

			//- poweruser has no access to kube-system secrets
			{
				msg:    "PowerUser has no read access to kube-system secrets",
				req:    req().ns("kube-system").verb("get").res("secrets").user("sszuecs").groups("PowerUser"),
				expect: deniedReason("unauthorized access", "sszuecs/[PowerUser]"),
			},

			//- poweruser can read secrets from non kube-system namespaces
			{
				msg:    "PowerUser has read access to non kube-system secrets",
				req:    req().ns("teapot").verb("get").res("secrets").user("sszuecs").groups("PowerUser"),
				expect: allowed,
			},

			//- poweruser has write access to non kube-system namespaces
			{
				msg:    "PowerUser has write access to non kube-system secrets",
				req:    req().ns("teapot").verb("create").res("secrets").user("sszuecs").groups("PowerUser"),
				expect: allowed,
			},

			//- TODO poweruser has exec right
			//- CHECK poweruser has proxy right
			{
				msg:    "PowerUser has proxy right",
				req:    req().ns("teapot").verb("create").res("pods/proxy").user("sszuecs").groups("PowerUser"),
				expect: allowed,
			},

			//- CHECK poweruser can not create daemonsets
			{
				msg:    "PowerUser has no create access to daemonsets",
				req:    req().ns("teapot").verb("create").res("daemonsets").user("sszuecs").groups("PowerUser"),
				expect: undecided,
			},

			//- CHECK poweruser can not update daemonsets
			{
				msg: "PowerUser has no update access to daemonsets",
				req: req().ns("teapot").verb("update").resGroup("apps").res("daemonsets").
					user("sszuecs").groups("PowerUser"),
				expect: undecided,
			},

			//- CHECK poweruser can not delete daemonsets
			{
				msg: "PowerUser has no delete access to daemonsets",
				req: req().ns("teapot").verb("delete").resGroup("apps").res("daemonsets").
					user("sszuecs").groups("PowerUser"),
				expect: undecided,
			},

			//- CHECK poweruser can not patch daemonsets
			{
				msg: "PowerUser has no patch access to daemonsets",
				req: req().ns("teapot").verb("patch").resGroup("apps").res("daemonsets").
					user("sszuecs").groups("PowerUser"),
				expect: undecided,
			},

			// poweruser can't delete metrics (non-resource endpoint)
			{
				msg:    "PowerUser can't delete metrics (non-resource endpoint access)",
				req:    req().verb("delete").path("/metrics").user("sszuecs").groups(powerUserGroup),
				expect: undecided,
			},

			//- operator is not allowed to use privileged PSP
			// Namespace is currently always empty string, because in Kubernetes PSPs are not namespaced, yet.
			// Check Kubernetes >= 1.7 if they namespaced it https://github.com/kubernetes/kubernetes/pull/42360
			{
				msg: "operator is not allowed to use privileged PodSecurityPolicy (for own namespace)",
				req: req().name("privileged").verb("user").resGroup("extensions").res("podsecuritypolicies").
					user("system:serviceaccount:teapot:operator"),
				expect: undecided,
			},

			//- operator has no read access to own namespace
			{
				msg: "operator has no read access to own namespace",
				req: req().ns("teapot").verb("get").res("pods").
					user("system:serviceaccount:teapot:operator"),
				expect: undecided,
			},

			//- operator has no write access to own namespace
			{
				msg: "operator has no write access to own namespace",
				req: req().ns("teapot").verb("create").res("pods").
					user("system:serviceaccount:teapot:operator"),
				expect: undecided,
			},

			//- operator has no read access to other namespaces
			{
				msg: "operator has no read access to other namespace",
				req: req().ns("coffeepot").verb("get").res("pods").
					user("system:serviceaccount:teapot:operator"),
				expect: undecided,
			},

			//- operator has no write access to other namespaces (not own)
			{
				msg: "operator has no write access to other namespace",
				req: req().ns("coffeepot").verb("create").res("pods").
					user("system:serviceaccount:teapot:operator"),
				expect: undecidedReason("access undecided system:serviceaccount:teapot:operator/[]"),
			},

			//- operator has no read access to secrets in own namespace
			{
				msg: "operator has read access to secrets in own namespace",
				req: req().ns("teapot").verb("get").res("secrets").
					user("system:serviceaccount:teapot:operator"),
				expect: undecided,
			},

			//- operator is not allowed to read secrets in other namespaces
			{
				msg: "operator is not allowed to read secrets in other namespaces",
				req: req().ns("coffeepot").verb("get").res("secrets").
					user("system:serviceaccount:teapot:operator"),
				expect: undecidedReason("access undecided system:serviceaccount:teapot:operator/[]"),
			},

			//- operator has no read access to custom resource definitions (CRD) in all namespaces
			{
				msg: "operator has read access to custom resource definitions (CRD) in all namespacese",
				req: req().verb("get").resGroup("apiextensions.k8s.io").res("customresourcedefinitions").
					user("system:serviceaccount:teapot:operator"),
				expect: undecided,
			},

			//- operator has no write access to custom resource definitions (CRD) in all namespaces
			{
				msg: "operator has read access to custom resource definitions (CRD) in all namespacese",
				req: req().verb("create").resGroup("apiextensions.k8s.io").res("customresourcedefinitions").
					user("system:serviceaccount:teapot:operator"),
				expect: undecided,
			},

			//- operator has no write access to storageclasses in all namespaces
			{
				msg: "operator has write access to storageclasses in all namespaces",
				req: req().verb("create").resGroup("storage.k8s.io").res("storageclasses").
					user("system:serviceaccount:teapot:operator"),
				expect: undecided,
			},

			//- operator has no read access to storageclasses in all namespaces
			{
				msg: "operator has read access to storageclasses in all namespaces",
				req: req().verb("get").resGroup("storage.k8s.io").res("storageclasses").
					user("system:serviceaccount:teapot:operator"),
				expect: undecided,
			},

			//- operator has no read access to nodes in global namespace
			{
				msg:    "operator has read access to nodes in global namespace",
				req:    req().verb("get").res("nodes").user("system:serviceaccount:teapot:operator"),
				expect: undecided,
			},

			//- operator has no write access to nodes in global namespace
			{
				msg:    "operator has write access to nodes in global namespace",
				req:    req().verb("create").res("nodes").user("system:serviceaccount:teapot:operator"),
				expect: undecided,
			},

			//- readonly is not allowed to read secrets all namespaces
			{
				msg:    "readonly is not allowed to read secrets all namespaces",
				req:    req().ns("coffeepot").verb("get").res("secrets").user("mkerk").groups("ReadOnly"),
				expect: undecidedReason("access undecided mkerk/[ReadOnly]"),
			},

			//- readonly is not allowed to use proxy
			{
				msg:    "readonly is not allowed to use proxy",
				req:    req().ns("coffeepot").verb("proxy").res("services").user("mkerk").groups("ReadOnly"),
				expect: undecidedReason("access undecided mkerk/[ReadOnly]"),
			},

			//- TODO: readonly is not allowed to use exec
			//- readonly has no write access to any resource
			{
				msg:    "readonly has no write access to any resource",
				req:    req().ns("coffeepot").verb("create").res("secrets").user("mkerk").groups("ReadOnly"),
				expect: undecidedReason("access undecided mkerk/[ReadOnly]"),
			},

			//- ReadOnly role cannot delete resources
			{
				msg:    "ReadOnly role cannot delete resources",
				req:    req().verb("delete").res("pods").user("rdifazio").groups("ReadOnly"),
				expect: undecidedReason("access undecided rdifazio/[ReadOnly]"),
			},

			//- Manual role can delete resources in all namespaces but kube-system
			{
				msg:    "Manual role can delete resources in all namespaces except kube-system",
				req:    req().verb("delete").res("pods").user("rdifazio").groups("ReadOnly", "Manual"),
				expect: allowed,
			},

			//- Manual role cannot delete resources in kube-sytem namespace
			{
				msg:    "Manual role cannot delete resources in kube-sytem namespace",
				req:    req().ns("kube-system").verb("delete").res("pods").user("rdifazio").groups("ReadOnly", "Manual"),
				expect: deniedReason("unauthorized access", "rdifazio/[ReadOnly Manual]"),
			},

			//- Manual role can delete namespaces
			{
				msg:    "Manual role can delete namespaces",
				req:    req().verb("delete").res("namespaces").user("rdifazio").groups("ReadOnly", "Manual"),
				expect: allowed,
			},

			//- Manual role can't delete kube-system namespace
			{
				msg:    "Manual role can't delete kube-system namespace",
				req:    req().name("kube-system").verb("delete").res("namespaces").user("rdifazio").groups("ReadOnly", "Manual"),
				expect: deniedReason("unauthorized access", "rdifazio/[ReadOnly Manual]"),
			},

			//- Manual role can create resources
			{
				msg:    "Manual role can create resources",
				req:    req().verb("create").res("pods").user("rdifazio").groups("ReadOnly", "Manual"),
				expect: allowed,
			},

			//- Manual role doesn't affect funtionality of other roles.
			{
				msg:    "Manual role doesn't affect funtionality of other roles.",
				req:    req().verb("get").res("pods").user("rdifazio").groups("ReadOnly", "Manual"),
				expect: allowed,
			},

			//- administrator can use restricted PSP
			{
				msg: "access to use PodSecurityPolicy for Administrator (system:masters) should be allowed",
				req: req().name("restricted").verb("user").resGroup("extensions").res("podsecuritypolicies").
					user("sszuecs").groups(systemMastersGroup),
				expect: allowed,
			},

			//- administrator can use privileged PSP
			{
				msg: "access to use PodSecurityPolicy for Administrator (system:masters) should be allowed",
				req: req().name("privileged").verb("use").resGroup("extensions").res("podsecuritypolicies").
					user("sszuecs").groups(systemMastersGroup),
				expect: allowed,
			},

			//- system:masters can use privileged PSP
			{
				msg: "access to use PodSecurityPolicy for system:masters should be allowed",
				req: req().name("privileged").verb("use").resGroup("extensions").res("podsecuritypolicies").
					user("sszuecs").groups(systemMastersGroup),
				expect: allowed,
			},

			//- Controller manager can list podsecurity policies
			{
				msg: "controller manager can list podsecurity policies",
				req: req().verb("list").resGroup("extensions").res("podsecuritypolicies").
					user("system:kube-controller-manager"),
				expect: allowed,
			},

			//- administrator has read access to kube system
			{
				msg:    "Administrator (system:masters) has read access (pods) to kube-system",
				req:    req().ns("kube-system").verb("get").res("pods").user("rdifazio").groups("system:masters"),
				expect: allowed,
			},

			//- administrator has write access to kube system
			{
				msg:    "Administrator (system:masters) has write access (pods) to kube-system",
				req:    req().ns("kube-system").verb("create").res("pods").user("rdifazio").groups("system:masters"),
				expect: allowed,
			},

			//- administrator can read secrets from kube-system namespaces
			{
				msg:    "Administrator (system:masters) can read secrets from kube-system namespaces",
				req:    req().ns("kube-system").verb("get").res("secrets").user("rdifazio").groups("system:masters"),
				expect: allowed,
			},

			//- administrator can read secrets from non kube-system namespaces
			{
				msg:    "Administrator (system:masters) can read secrets from non kube-system namespaces",
				req:    req().ns("teapot").verb("get").res("secrets").user("rdifazio").groups("system:masters"),
				expect: allowed,
			},

			//- administrator has write access to non kube-system namespaces
			{
				msg:    "Administrator (system:masters) has write access to non kube-system namespaces",
				req:    req().ns("teapot").verb("create").res("pods").user("rdifazio").groups("system:masters"),
				expect: allowed,
			},
			//- TODO administrator has exec right

			//- administrator has proxy right
			{
				msg:    "Administrator (system:masters) has proxy right",
				req:    req().ns("teapot").verb("proxy").user("sszuecs").groups("system:masters"),
				expect: allowed,
			},

			//- administrator can write daemonsets
			{
				msg: "Administrator (system:masters) can write daemonsets",
				req: req().ns("teapot").verb("create").resGroup("apps").res("daemonsets").
					user("sszuecs").groups("system:masters"),
				expect: allowed,
			},

			{
				msg:    "cdp service account can create namespaces",
				req:    req().verb("create").res("namespaces").user("system:serviceaccount:default:cdp"),
				expect: allowed,
			},

			{
				msg: "cdp service account can't escalate permissions",
				req: req().verb("escalate").resGroup("rbac.authorization.k8s.io").res("clusterroles").
					user("system:serviceaccount:default:cdp"),
				expect: denied,
			},

			{
				msg: "PowerUsers can't escalate permissions",
				req: req().verb("escalate").resGroup("rbac.authorization.k8s.io").res("clusterroles").
					user("mlarsen").groups("PowerUser"),
				expect: denied,
			},

			{
				msg:    "operator service account cannot create namespaces",
				req:    req().verb("create").res("namespaces").user("system:serviceaccount:default:operator"),
				expect: undecidedReason("access undecided system:serviceaccount:default:operator/[]"),
			},

			{
				msg: "controller manager service account can create pods",
				req: req().ns("kube-system").verb("create").res("pods").
					user("system:serviceaccount:kube-system:daemon-set-controller").
					groups("system:serviceaccounts:kube-system"),
				expect: allowed,
			},

			{
				msg: "operator service account can not access persistent volumes in other namespaces",
				req: req().verb("get").res("persistentvolumes").
					user("system:serviceaccount:default:operator"),
				expect: undecided,
			},

			{
				msg: "persistent volume binder service account can update kube system persistentVolumeClaims",
				req: req().ns("kube-system").verb("update").res("persistentvolumeclaims").
					user("system:serviceaccount:kube-system:persistent-volume-binder").
					groups("system:serviceaccounts:kube-system"),
				expect: allowed,
			},

			{
				msg: "persistent volume binder service account can create kube system persistentVolumes",
				req: req().ns("kube-system").verb("create").res("persistentvolumes").
					user("system:serviceaccount:kube-system:persistent-volume-binder").
					groups("system:serviceaccounts:kube-system"),
				expect: allowed,
			},

			{
				msg: "horizontal pod autoscaler service account can update kube system autoscalers",
				req: req().ns("kube-system").verb("update").resGroup("*").res("*/scale").
					user("system:serviceaccount:kube-system:horizontal-pod-autoscaler").
					groups("system:serviceaccounts:kube-system"),
				expect: allowed,
			},

			{
				msg: "horizontal pod autoscaler service account can update any autoscaler",
				req: req().ns("*").verb("update").resGroup("*").res("*/scale").
					user("system:serviceaccount:kube-system:horizontal-pod-autoscaler").
					groups("system:serviceaccounts:kube-system"),
				expect: allowed,
			},

			{
				msg: "aws-cloud-provider service account can access patch nodes",
				req: req().verb("patch").res("nodes").
					user("system:serviceaccount:kube-system:aws-cloud-provider").
					groups("system:serviceaccounts:kube-system"),
				expect: allowed,
			},

			{
				msg:    "emergency user should not have update access to node resources.",
				req:    req().verb("update").res("nodes").user("sszuecs").groups(emergencyGroup),
				expect: undecided,
			},

			{
				msg:    "manual user should not have non update to node resources.",
				req:    req().verb("update").res("nodes").user("sszuecs").groups(manualGroup),
				expect: undecided,
			},

			{
				msg:    "power user should not have update access to node resources.",
				req:    req().verb("update").res("nodes").user("sszuecs").groups(powerUserGroup),
				expect: undecided,
			},

			{
				msg:    "emergency user should not have create access to node resources.",
				req:    req().verb("create").res("nodes").user("sszuecs").groups(emergencyGroup),
				expect: undecided,
			},

			{
				msg:    "manual user should not have create access to node resources.",
				req:    req().verb("create").res("nodes").user("sszuecs").groups(manualGroup),
				expect: undecided,
			},

			{
				msg:    "power user should not have create access to node resources.",
				req:    req().verb("create").res("nodes").user("sszuecs").groups(powerUserGroup),
				expect: undecided,
			},

			{
				msg:    "emergency user should not have patch access to node resources.",
				req:    req().verb("patch").res("nodes").user("sszuecs").groups(emergencyGroup),
				expect: undecided,
			},

			{
				msg:    "manual user should not have patch access to node resources.",
				req:    req().verb("patch").res("nodes").user("sszuecs").groups(manualGroup),
				expect: undecided,
			},

			{
				msg:    "power user should not have patch access to node resources.",
				req:    req().verb("patch").res("nodes").user("sszuecs").groups(powerUserGroup),
				expect: undecided,
			},

			{
				msg:    "emergency user should not have delete access to node resources.",
				req:    req().verb("delete").res("nodes").user("sszuecs").groups(emergencyGroup),
				expect: undecided,
			},

			{
				msg:    "manual user should not have delete access to node resources.",
				req:    req().verb("delete").res("nodes").user("sszuecs").groups(manualGroup),
				expect: undecided,
			},

			{
				msg:    "power user should not have delete access to node resources.",
				req:    req().verb("delete").res("nodes").user("sszuecs").groups(powerUserGroup),
				expect: undecided,
			},

			{
				msg:    "power user should be allowed list access to node resources.",
				req:    req().verb("list").res("nodes").user("sszuecs").groups(powerUserGroup),
				expect: allowed,
			},

			{
				msg:    "emergency user should be allowed list access to node resources.",
				req:    req().verb("list").res("nodes").user("sszuecs").groups(emergencyGroup),
				expect: allowed,
			},

			{
				msg:    "manual user should be allowed list access to node resources.",
				req:    req().verb("list").res("nodes").user("sszuecs").groups(manualGroup),
				expect: allowed,
			},

			{
				msg:    "power user should be allowed read access to node resources.",
				req:    req().verb("get").res("nodes").user("sszuecs").groups(powerUserGroup),
				expect: allowed,
			},

			{
				msg:    "emergency user should be allowed read access to node resources.",
				req:    req().verb("get").res("nodes").user("sszuecs").groups(emergencyGroup),
				expect: allowed,
			},

			{
				msg:    "manual user should be allowed read access to node resources.",
				req:    req().verb("get").res("nodes").user("sszuecs").groups(manualGroup),
				expect: allowed,
			},

			{
				msg: "system user (credentials-provider) should be allowed get secrets in kube-system.",
				req: req().ns("kube-system").verb("get").res("secrets").
					user("zalando-iam:zalando:service:credentials-provider"),
				expect: allowed,
			},

			{
				msg: "non system user (cdp-controller) should NOT be allowed get secrets in kube-system.",
				req: req().ns("kube-system").verb("get").res("secrets").
					user("zalando-iam:zalando:service:credprov-cdp-controller-cluster-token"),
				expect: deniedReason("unauthorized access to system namespace by zalando-iam:zalando:service:credprov-cdp-controller-cluster-token/[]"),
			},

			{
				msg: "system user (api-monitoring-controller) can update configmap 'skipper-default-filters' in kube-system.",
				req: req().ns("kube-system").verb("update").res("configmaps").name("skipper-default-filters").
					user("system:serviceaccount:api-infrastructure:api-monitoring-controller"),
				expect: allowed,
			},

			{
				msg: "system user (api-monitoring-controller) can NOT update any configmap in kube-system.",
				req: req().ns("kube-system").verb("update").res("configmaps").
					user("system:serviceaccount:api-infrastructure:api-monitoring-controller"),
				expect: undecidedReason("undecided system:serviceaccount:api-infrastructure:api-monitoring-controller/[]"),
			},
		} {

			By(ti.msg)

			req, err := makeReq(ti.req)
			Expect(err).NotTo(HaveOccurred())

			resp, err := client.Do(req)
			Expect(err).NotTo(HaveOccurred())

			body, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())

			if resp.StatusCode != ti.expect.status {
				framework.Failf(
					"%s: invalid status code received. expected %d, got %d\n%s",
					ti.msg,
					ti.expect.status,
					resp.StatusCode,
					string(body),
				)

				return
			}

			var authzResp authorizationResp
			if err := json.Unmarshal(body, &authzResp); err != nil && err != io.EOF {
				framework.Failf(ti.msg, err)
				return
			}

			if authzResp.Status.Allowed != ti.expect.allowed || authzResp.Status.Denied != ti.expect.denied {
				framework.Failf(
					"unexpected response. expected %v, got %v",
					ti.expect,
					response{
						status:  resp.StatusCode,
						allowed: authzResp.Status.Allowed,
						denied:  authzResp.Status.Denied,
						reason:  []string{authzResp.Status.Reason},
					},
				)
			}

			for _, r := range ti.expect.reason {
				if !strings.Contains(authzResp.Status.Reason, r) {
					framework.Failf(
						"expected reason not found: %s, got instead: %s",
						r,
						authzResp.Status.Reason,
					)
				}
			}
		}
	})
})
