package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	powerUserGroup  = "PowerUser"
	emergencyGroup  = "Emergency"
	manualGroup     = "Manual"
	readOnlyGroup   = "ReadOnly"
	accessReviewURL = "/apis/authorization.k8s.io/v1/subjectaccessreviews"
)

type apiHeader struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
}

type resourceAttributes struct {
	Namespace   string `json:"namespace,omitempty"`
	Name        string `json:"name,omitempty"`
	Verb        string `json:"verb,omitempty"`
	Group       string `json:"group,omitempty"`
	Resource    string `json:"resource,omitempty"`
	Subresource string `json:"subresource,omitempty"`
	Path        string `json:"path,omitempty"`
}

type nonResourceAttributes struct {
	Verb string `json:"verb,omitempty"`
	Path string `json:"path,omitempty"`
}

type subjectReviewSpec struct {
	ResourceAttributes    *resourceAttributes    `json:"resourceAttributes,omitempty"`
	NonResourceAttributes *nonResourceAttributes `json:"nonResourceAttributes,omitempty"`
	User                  string                 `json:"user,omitempty"`
	Groups                []string               `json:"groups,omitempty"`
}

type subjectReview struct {
	apiHeader
	Spec subjectReviewSpec `json:"spec"`
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

type requestData struct {
	namespaces       []string
	names            []string
	verbs            []string
	apiGroups        []string
	resources        []string
	subresources     []string
	paths            []string
	nonResourceVerbs []string
	nonResourcePaths []string
	users            []string
	groups           [][]string
}

type testItem struct {
	name    string
	request requestData
	items   []testItem
	expect  response
}

func (item testItem) expandOn(subitems []testItem, field func(*testItem) *[]string) []testItem {
	values := field(&item)
	if len(*values) == 0 {
		return subitems
	}

	var expanded []testItem
	for _, subitem := range subitems {
		// == 1:
		// - if it's coming from a lower level, then it's either zero or one, and is considered
		// as overriding the current level.
		// - if it's the item from the current level, then no need to expand when there's only one
		// value.
		if len(*field(&subitem)) == 1 {
			expanded = append(expanded, subitem)
			continue
		}

		for _, value := range *values {
			copy := subitem
			copyField := field(&copy)
			*copyField = []string{value}
			expanded = append(expanded, copy)
		}
	}

	return expanded
}

func (item testItem) expandOnGroups(subitems []testItem) []testItem {
	if len(item.request.groups) == 0 {
		return subitems
	}

	var expanded []testItem
	for _, subitem := range subitems {
		if len(subitem.request.groups) == 1 {
			expanded = append(expanded, subitem)
			continue
		}

		for _, groupSet := range item.request.groups {
			copy := subitem
			copy.request.groups = [][]string{groupSet}
			expanded = append(expanded, copy)
		}
	}

	return expanded
}

func (item testItem) expand() []testItem {
	var all []testItem
	if len(item.items) == 0 {
		all = append(all, item)
	} else {
		for _, subitem := range item.items {
			all = append(all, subitem.expand()...)
		}

		for i := range all {
			all[i].name = fmt.Sprintf("%s/%s", item.name, all[i].name)
		}
	}

	all = item.expandOn(all, func(item *testItem) *[]string { return &item.request.namespaces })
	all = item.expandOn(all, func(item *testItem) *[]string { return &item.request.names })
	all = item.expandOn(all, func(item *testItem) *[]string { return &item.request.verbs })
	all = item.expandOn(all, func(item *testItem) *[]string { return &item.request.apiGroups })
	all = item.expandOn(all, func(item *testItem) *[]string { return &item.request.resources })
	all = item.expandOn(all, func(item *testItem) *[]string { return &item.request.subresources })
	all = item.expandOn(all, func(item *testItem) *[]string { return &item.request.paths })
	all = item.expandOn(all, func(item *testItem) *[]string { return &item.request.nonResourceVerbs })
	all = item.expandOn(all, func(item *testItem) *[]string { return &item.request.nonResourcePaths })
	all = item.expandOn(all, func(item *testItem) *[]string { return &item.request.users })
	all = item.expandOnGroups(all)

	for i := range all {
		if all[i].expect.status == 0 {
			all[i].expect = item.expect
		}
	}

	return all
}

func req() requestData { return requestData{} }

func (r requestData) ns(ns ...string) requestData {
	r.namespaces = ns
	return r
}

func (r requestData) name(n ...string) requestData {
	r.names = n
	return r
}

func (r requestData) verb(v ...string) requestData {
	r.verbs = v
	return r
}

func (r requestData) apiGroup(g ...string) requestData {
	r.apiGroups = g
	return r
}

func (r requestData) res(res ...string) requestData {
	r.resources = res
	return r
}

func (r requestData) subres(subres ...string) requestData {
	r.subresources = subres
	return r
}

func (r requestData) path(p ...string) requestData {
	r.paths = p
	return r
}

func (r requestData) nonResVerb(v ...string) requestData {
	r.nonResourceVerbs = v
	return r
}

func (r requestData) nonResPath(p ...string) requestData {
	r.nonResourcePaths = p
	return r
}

func (r requestData) user(u ...string) requestData {
	r.users = u
	return r
}

func (r requestData) setGroups(g ...[]string) requestData {
	r.groups = g
	return r
}

func (item testItem) subjectReview() subjectReview {
	req := subjectReview{
		apiHeader: apiHeader{
			APIVersion: "authorization.k8s.io/v1",
			Kind:       "SubjectAccessReview",
		},
	}

	// taking the first value if exists, because at this point the test item should be
	// already expanded
	setIfExists := func(field *string, values []string) {
		if len(values) > 0 {
			*field = values[0]
		}
	}

	if len(item.request.nonResourceVerbs) > 0 || len(item.request.nonResourcePaths) > 0 {
		req.Spec.NonResourceAttributes = &nonResourceAttributes{}
		setIfExists(&req.Spec.NonResourceAttributes.Verb, item.request.nonResourceVerbs)
		setIfExists(&req.Spec.NonResourceAttributes.Path, item.request.nonResourcePaths)
	} else {
		req.Spec.ResourceAttributes = &resourceAttributes{}
		setIfExists(&req.Spec.ResourceAttributes.Namespace, item.request.namespaces)
		setIfExists(&req.Spec.ResourceAttributes.Name, item.request.names)
		setIfExists(&req.Spec.ResourceAttributes.Verb, item.request.verbs)
		setIfExists(&req.Spec.ResourceAttributes.Group, item.request.apiGroups)
		setIfExists(&req.Spec.ResourceAttributes.Resource, item.request.resources)
		setIfExists(&req.Spec.ResourceAttributes.Subresource, item.request.subresources)
		setIfExists(&req.Spec.ResourceAttributes.Path, item.request.paths)

		parts := strings.Split(req.Spec.ResourceAttributes.Resource, "/")
		switch {
		case len(parts) == 2:
			req.Spec.ResourceAttributes.Group = parts[0]
			req.Spec.ResourceAttributes.Resource = parts[1]
		case len(parts) == 3:
			req.Spec.ResourceAttributes.Group = parts[0]
			req.Spec.ResourceAttributes.Resource = parts[1]
			req.Spec.ResourceAttributes.Subresource = parts[2]
		}
	}

	setIfExists(&req.Spec.User, item.request.users)
	if len(item.request.groups) > 0 {
		req.Spec.Groups = item.request.groups[0]
	}

	return req
}

func (item testItem) String() string {
	var attr []string
	addIfExists := func(fields [][]string) {
		for _, f := range fields {
			if len(f) > 0 && f[0] != "" {
				attr = append(attr, f[0])
			}
		}
	}

	if len(item.request.nonResourceVerbs) > 0 || len(item.request.nonResourcePaths) > 0 {
		addIfExists([][]string{
			item.request.nonResourceVerbs,
			item.request.nonResourcePaths,
			item.request.users,
		})
	} else {
		addIfExists([][]string{
			item.request.namespaces,
			item.request.names,
			item.request.verbs,
			item.request.apiGroups,
			item.request.resources,
			item.request.subresources,
			item.request.paths,
			item.request.users,
		})
	}

	if len(item.request.groups) > 0 {
		attr = append(attr, fmt.Sprint(item.request.groups[0]))
	}

	return fmt.Sprintf("%s - %v", item.name, attr)
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

func newReqBuilder(url, token string) func(subjectReview) (*http.Request, error) {
	return func(body subjectReview) (*http.Request, error) {
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

func verifyResponse(status int, body []byte, test testItem) {
	if status != test.expect.status {
		framework.Failf(
			"%s: invalid status code received. expected %d, got %d\n%s",
			test.name,
			test.expect.status,
			status,
			string(body),
		)

		return
	}

	var authzResp authorizationResp
	if err := json.Unmarshal(body, &authzResp); err != nil && err != io.EOF {
		framework.Failf(test.name, err)
		return
	}

	// undecided is considered as denied
	if authzResp.Status.Allowed != test.expect.allowed ||
		test.expect.denied && authzResp.Status.Allowed {
		framework.Failf(
			"unexpected response. expected %v, got %v",
			test.expect,
			response{
				status:  status,
				allowed: authzResp.Status.Allowed,
				denied:  authzResp.Status.Denied,
				reason:  []string{authzResp.Status.Reason},
			},
		)
	}

	for _, r := range test.expect.reason {
		if !strings.Contains(authzResp.Status.Reason, r) {
			framework.Failf(
				"expected reason not found: %s, got instead: %s",
				r,
				authzResp.Status.Reason,
			)
		}
	}
}

var _ = framework.KubeDescribe("Authorization tests [Authorization] [RBAC] [Zalando]", func() {
	should := fmt.Sprintf(
		"should validate permissions for [Authorization] [RBAC] [Zalando]",
	)
	It(should, func() {
		conf, err := framework.LoadConfig()
		Expect(err).NotTo(HaveOccurred()) // BDD = Because :DDD

		host := conf.Host
		client := http.DefaultClient
		makeReq := newReqBuilder(host+accessReviewURL, conf.BearerToken)

		for _, test := range []testItem{{
			name: "everyone",
			request: req().user("test-user").
				setGroups(
					[]string{"FooBar"},
					[]string{"ReadOnly"},
					[]string{"PowerUser"},
					[]string{"Emergency"},
					[]string{"Manual"},
					[]string{"system:serviceaccounts:kube-system"},
					[]string{"CollaboratorEmergency"},
					[]string{"CollaboratorManual"},
					[]string{"Collaborator24x7"},
					[]string{"CollaboratorPowerUser"},
					[]string{"Administrator"},
				),
			items: []testItem{{
				name:    "impersonate denied",
				request: req().verb("impersonate"),
				expect:  denied,
				items: []testItem{{
					name:    "users and groups",
					request: req().res("users", "groups"),
				}, {
					name: "service accounts, namespaced",
					request: req().
						res("serviceaccounts").
						ns("", "teapot", "kube-system"),
				}},
			}, {
				name:    "escalate denided",
				request: req().verb("escalate"),
				expect:  denied,
				items: []testItem{{
					name:    "cluster role",
					request: req().res("rbac.authorization.k8s.io/clusterrole"),
				}, {
					name: "role",
					request: req().res("rbac.authorization.k8s.io/role").
						ns("", "teapot", "kube-system"),
				}},
			}},
		}, {

			name:    "read-only users",
			request: req().user("test-user").setGroups([]string{"ReadOnly"}),
			items: []testItem{{
				name:   "no access to secrets",
				expect: denied,
				request: req().res("secrets").ns("", "teapot", "kube-system").verb(
					"get",
					"list",
					"watch",
					"create",
					"patch",
					"update",
					"delete",
				),
			}, {
				name: "other resources",
				items: []testItem{{
					name: "namespaced",
					request: req().ns("default", "teapot", "kube-system").res(
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
					),
					items: []testItem{{
						name:    "no write access",
						request: req().verb("create", "patch", "update", "delete"),
						expect:  denied,
					}, {
						name:    "read access",
						request: req().verb("get", "list", "watch"),
						expect:  allowed,
					}},
				}, {
					name: "not namespaced",
					request: req().res(
						"namespaces",
						"nodes",
						"rbac.authorization.k8s.io/clusterroles",
						"storage.k8s.io/storageclasses",
						"policy/podsecuritypolicies",
						"apiextensions.k8s.io/customresourcedefinitions",
					),
					items: []testItem{{
						name:    "no write access",
						request: req().verb("create", "patch", "update", "delete"),
						expect:  denied,
					}, {
						name:    "read access",
						request: req().verb("get", "list", "watch"),
						expect:  allowed,
					}},
				}},
			}},
		}, {

			name: "power-user, manual, emergency",
			request: req().
				user("test-user").
				setGroups([]string{"PowerUser"}, []string{"Manual"}, []string{"Emergency"}),
			items: []testItem{{
				name: "no access to secrets in kube-system or visibility",
				request: req().
					verb("get", "list", "watch").
					ns("kube-system", "visibility").
					res("secrets"),
				expect: denied,
			}, {
				name: "no write access to nodes",
				request: req().
					verb("create", "patch", "update", "delete").
					res("nodes"),
				expect: denied,
			}, {
				name: "no write to daemonsets",
				request: req().
					verb("create", "patch", "update", "delete").
					ns("", "teapot", "kube-system").
					res("apps/daemonsets"),
				expect: denied,
			}, {

				name: "delete of CRDs",
				request: req().
					verb("delete").
					res("apiextensions.k8s.io/customresourcedefinitions"),
				expect: allowed,
			}, {

				name: "no delete of kube-system or visibility namespaces",
				request: req().
					verb("delete").
					name("kube-system", "visibility"),
				expect: denied,
			}, {

				name: "write access to everything, except kube-system and visibility",
				request: req().
					verb("create", "patch", "update", "delete"),
				items: []testItem{{
					name: "namespaced",
					request: req().res(
						"pods",
						"apps/deployments",
						"apps/statefulsets",
						"apps/deployments/scale",
						"apps/statefulsets/scale",
						"services",
						"persistentvolumes",
						"persistentvolumeclaims",
						"configmaps",
					),
					items: []testItem{{
						name:    "kube-system and visibility",
						request: req().ns("kube-system", "visibility"),
						expect:  denied,
					}, {
						name:    "others",
						request: req().ns("", "teapot"),
						expect:  allowed,
					}},
				}, {
					name: "not namespaced",
					items: []testItem{{
						name: "allowed",
						request: req().res(
							"namespaces",
							"storage.k8s.io/storageclasses",
							"apiextensions.k8s.io/customresourcedefinitions",
						),
						expect: allowed,
					}, {
						name: "not allowed",
						request: req().res(
							"nodes",
							"policy/podsecuritypolicies",
						),
						expect: denied,
					}},
				}},
			}},
		}, {

			name: "collaborator power-user, manual and emergency",
			request: req().
				user("test-user").
				setGroups(
					[]string{"CollaboratorPowerUser", "PowerUser"},
					[]string{"CollaboratorManual", "Manual"},
					[]string{"CollaboratorEmergency", "Emergency"},
				),
			items: []testItem{{

				name: "access to secrets in kube-system or visibility",
				request: req().
					verb("get", "list", "watch").
					ns("kube-system", "visibility").
					res("secrets"),
				items: []testItem{{
					name:    "in visibility",
					request: req().ns("visibility"),
					expect:  allowed,
				}, {
					name:    "in kube-system",
					request: req().ns("kube-system"),
					expect:  denied,
				}},
			}, {
				name: "no write access to nodes",
				request: req().
					verb("create", "patch", "update", "delete").
					res("nodes"),
				expect: denied,
			}, {
				name: "can update to daemonsets",
				request: req().
					verb("create", "patch", "update", "delete").
					ns("visibility").
					res("apps/daemonsets"),
				expect: allowed,
			}, {

				name: "delete of CRDs",
				request: req().
					verb("delete").
					res("apiextensions.k8s.io/customresourcedefinitions"),
				expect: allowed,
			}, {

				name: "no delete of kube-system or visibility namespaces",
				request: req().
					verb("delete").
					res("namespaces").
					name("kube-system", "visibility"),
				expect: denied,
			}, {

				name: "write access to everything, except kube-system",
				request: req().
					verb("create", "patch", "update", "delete"),
				items: []testItem{{
					name: "namespaced",
					request: req().res(
						"pods",
						"apps/deployments",
						"apps/statefulsets",
						"services",
						"persistentvolumes",
						"persistentvolumeclaims",
						"configmaps",
					),
					items: []testItem{{
						name:    "kube-system and visibility",
						request: req().ns("kube-system"),
						expect:  denied,
					}, {
						name:    "others",
						request: req().ns("", "teapot", "visibility"),
						expect:  allowed,
					}},
				}, {
					name: "not namespaced",
					items: []testItem{{
						name: "allowed",
						request: req().res(
							"namespaces",
							"storage.k8s.io/storageclasses",
							"apiextensions.k8s.io/customresourcedefinitions",
						),
						expect: allowed,
					}, {
						name: "not allowed",
						request: req().res(
							"nodes",
							"policy/podsecuritypolicies",

							// "rbac.authorization.k8s.io/clusterroles",
						),
						expect: denied,
					}},
				}},
			}},
		}, {

			name: "system",
			items: []testItem{{
				name: "kubelet authorized",
				request: req().ns("teapot").verb("get").res("pods").
					user("kubelet").setGroups([]string{"system:masters"}),
				expect: allowed,
			}, {
				name: "kube-system daemonset-controller service account can update daemonset status",
				request: req().ns("kube-system").verb("update").apiGroup("extensions").res("daemonsets").subres("status").
					user("system:serviceaccount:kube-system:daemon-set-controller").
					setGroups([]string{"system:serviceaccounts:kube-system"}),
				expect: allowed,
			}, {
				name: "kube-system default account can update daemonset finalizers",
				request: req().ns("kube-system").verb("update").apiGroup("extensions").res("daemonsets").subres("finalizers").
					user("system:serviceaccount:kube-system:daemon-set-controller").
					setGroups([]string{"system:serviceaccounts:kube-system"}),
				expect: allowed,
			}, {
				name:    "default account in default namespace can not list statefulsets",
				request: req().verb("list").res("statefulsets").user("system:serviceaccount:default:default"),
				expect:  denied,
			}, {
				name: "default account in non-default namespace can not list statefulsets",
				request: req().ns("non-default").verb("list").res("statefulsets").
					user("system:serviceaccount:non-default:default"),
				expect: denied,
			}, {
				name: "User in admin group can patch daemonsets",
				request: req().ns("kube-system").name("prometheus-node-exporter").
					verb("patch").apiGroup("extensions").res("daemonsets").
					user("sszuecs").
					setGroups([]string{"ReadOnly", "system:masters", "system:authenticated"}),
				expect: allowed,
			}, {
				name: "controller manager can list podsecurity policies",
				request: req().verb("list").apiGroup("extensions").res("podsecuritypolicies").
					user("system:kube-controller-manager"),
				expect: allowed,
			}, {
				name: "controller manager service account can create pods",
				request: req().ns("kube-system").verb("create").res("pods").
					user("system:serviceaccount:kube-system:daemon-set-controller").
					setGroups([]string{"system:serviceaccounts:kube-system"}),
				expect: allowed,
			}, {
				name: "persistent volume binder service account can update kube system persistentVolumeClaims",
				request: req().ns("kube-system").verb("update").res("persistentvolumeclaims").
					user("system:serviceaccount:kube-system:persistent-volume-binder").
					setGroups([]string{"system:serviceaccounts:kube-system"}),
				expect: allowed,
			}, {
				name: "persistent volume binder service account can create kube system persistentVolumes",
				request: req().ns("kube-system").verb("create").res("persistentvolumes").
					user("system:serviceaccount:kube-system:persistent-volume-binder").
					setGroups([]string{"system:serviceaccounts:kube-system"}),
				expect: allowed,
			}, {
				name: "aws-cloud-provider service account can access patch nodes",
				request: req().verb("patch").res("nodes").
					user("system:serviceaccount:kube-system:aws-cloud-provider").
					setGroups([]string{"system:serviceaccounts:kube-system"}),
				expect: allowed,
			}, {
				name: "system user (credentials-provider) should be allowed get secrets in kube-system.",
				request: req().ns("kube-system").verb("get").res("secrets").
					user("zalando-iam:zalando:service:credentials-provider"),
				expect: allowed,
			}, {
				name: "system user (api-monitoring-controller) can update configmap 'skipper-default-filters' in kube-system.",
				request: req().ns("kube-system").verb("update").res("configmaps").name("skipper-default-filters").
					user("system:serviceaccount:api-infrastructure:api-monitoring-controller"),
				expect: allowed,
			}, {
				name: "system user (api-monitoring-controller) can NOT update any configmap in kube-system.",
				request: req().ns("kube-system").verb("update").res("configmaps").
					user("system:serviceaccount:api-infrastructure:api-monitoring-controller"),
				expect: undecidedReason("undecided system:serviceaccount:api-infrastructure:api-monitoring-controller/[]"),
			}},
		}, {

			name: "operators",
			items: []testItem{{
				name: "operator is not allowed to use privileged PodSecurityPolicy (for own namespace)",
				request: req().name("privileged").verb("use").apiGroup("extensions").res("podsecuritypolicies").
					user("system:serviceaccount:teapot:operator"),
				expect: undecided,
			}, {
				name: "operator has no read access to own namespace",
				request: req().ns("teapot").verb("get").res("pods").
					user("system:serviceaccount:teapot:operator"),
				expect: undecided,
			}, {
				name: "operator has no write access to own namespace",
				request: req().ns("teapot").verb("create").res("pods").
					user("system:serviceaccount:teapot:operator"),
				expect: undecided,
			}, {
				name: "operator has no read access to other namespace",
				request: req().ns("coffeepot").verb("get").res("pods").
					user("system:serviceaccount:teapot:operator"),
				expect: undecided,
			}, {
				name: "operator has no write access to other namespace",
				request: req().ns("coffeepot").verb("create").res("pods").
					user("system:serviceaccount:teapot:operator"),
				expect: undecidedReason("access undecided system:serviceaccount:teapot:operator/[]"),
			}, {
				name: "operator has read access to secrets in own namespace",
				request: req().ns("teapot").verb("get").res("secrets").
					user("system:serviceaccount:teapot:operator"),
				expect: undecided,
			}, {
				name: "operator is not allowed to read secrets in other namespaces",
				request: req().ns("coffeepot").verb("get").res("secrets").
					user("system:serviceaccount:teapot:operator"),
				expect: undecidedReason("access undecided system:serviceaccount:teapot:operator/[]"),
			}, {
				name: "operator has read access to custom resource definitions (CRD) in all namespacese",
				request: req().verb("get").apiGroup("apiextensions.k8s.io").res("customresourcedefinitions").
					user("system:serviceaccount:teapot:operator"),
				expect: undecided,
			}, {
				name: "operator has read access to custom resource definitions (CRD) in all namespacese",
				request: req().verb("create").apiGroup("apiextensions.k8s.io").res("customresourcedefinitions").
					user("system:serviceaccount:teapot:operator"),
				expect: undecided,
			}, {
				name: "operator has write access to storageclasses in all namespaces",
				request: req().verb("create").apiGroup("storage.k8s.io").res("storageclasses").
					user("system:serviceaccount:teapot:operator"),
				expect: undecided,
			}, {
				name: "operator has read access to storageclasses in all namespaces",
				request: req().verb("get").apiGroup("storage.k8s.io").res("storageclasses").
					user("system:serviceaccount:teapot:operator"),
				expect: undecided,
			}, {
				name:    "operator has read access to nodes in global namespace",
				request: req().verb("get").res("nodes").user("system:serviceaccount:teapot:operator"),
				expect:  undecided,
			}, {
				name:    "operator has write access to nodes in global namespace",
				request: req().verb("create").res("nodes").user("system:serviceaccount:teapot:operator"),
				expect:  undecided,
			}, {
				name:    "operator service account cannot create namespaces",
				request: req().verb("create").res("namespaces").user("system:serviceaccount:default:operator"),
				expect:  undecidedReason("access undecided system:serviceaccount:default:operator/[]"),
			}, {
				name: "operator service account can not access persistent volumes in other namespaces",
				request: req().verb("get").res("persistentvolumes").
					user("system:serviceaccount:default:operator"),
				expect: undecided,
			}},
		}, {

			name: "administrator",
			items: []testItem{{
				name: "access to use PodSecurityPolicy for Administrator (system:masters) should be allowed",
				request: req().name("restricted").verb("use").apiGroup("extensions").res("podsecuritypolicies").
					user("sszuecs").setGroups([]string{"system:masters"}),
				expect: allowed,
			}, {
				name: "access to use PodSecurityPolicy for Administrator (system:masters) should be allowed",
				request: req().name("privileged").verb("use").apiGroup("extensions").res("podsecuritypolicies").
					user("sszuecs").setGroups([]string{"system:masters"}),
				expect: allowed,
			}, {
				name: "access to use PodSecurityPolicy for system:masters should be allowed",
				request: req().name("privileged").verb("use").apiGroup("extensions").res("podsecuritypolicies").
					user("sszuecs").setGroups([]string{"system:masters"}),
				expect: allowed,
			}, {
				name: "Administrator (system:masters) has read access (pods) to kube-system",
				request: req().ns("kube-system").verb("get").res("pods").
					user("rdifazio").setGroups([]string{"system:masters"}),
				expect: allowed,
			}, {
				name: "Administrator (system:masters) has write access (pods) to kube-system",
				request: req().ns("kube-system").verb("create").res("pods").
					user("rdifazio").setGroups([]string{"system:masters"}),
				expect: allowed,
			}, {
				name: "Administrator (system:masters) can read secrets from kube-system namespaces",
				request: req().ns("kube-system").verb("get").res("secrets").
					user("rdifazio").setGroups([]string{"system:masters"}),
				expect: allowed,
			}, {
				name: "Administrator (system:masters) can read secrets from non kube-system namespaces",
				request: req().ns("teapot").verb("get").res("secrets").
					user("rdifazio").setGroups([]string{"system:masters"}),
				expect: allowed,
			}, {
				name: "Administrator (system:masters) has write access to non kube-system namespaces",
				request: req().ns("teapot").verb("create").res("pods").
					user("rdifazio").setGroups([]string{"system:masters"}),
				expect: allowed,
			}, {
				name: "Administrator (system:masters) has proxy right",
				request: req().ns("teapot").verb("proxy").
					user("sszuecs").setGroups([]string{"system:masters"}),
				expect: allowed,
			}, {
				name: "Administrator (system:masters) can write daemonsets",
				request: req().ns("teapot").verb("create").apiGroup("apps").res("daemonsets").
					user("sszuecs").setGroups([]string{"system:masters"}),
				expect: allowed,
			}},
		}, {
			name: "CDP",
			items: []testItem{{
				name: "non system user (cdp-controller) should NOT be allowed get secrets in kube-system.",
				request: req().ns("kube-system").verb("get").res("secrets").
					user("zalando-iam:zalando:service:stups_cdp-controller"),
				expect: deniedReason("unauthorized access to system namespace by zalando-iam:zalando:service:stups_cdp-controller/[]"),
			}},
		},
		} {
			for _, subtest := range test.expand() {
				By(subtest.String())

				req, err := makeReq(subtest.subjectReview())
				Expect(err).NotTo(HaveOccurred())
				rsp, err := client.Do(req)
				Expect(err).NotTo(HaveOccurred())

				body, err := ioutil.ReadAll(rsp.Body)
				Expect(err).NotTo(HaveOccurred())

				verifyResponse(rsp.StatusCode, body, subtest)
			}
		}
	})
})
