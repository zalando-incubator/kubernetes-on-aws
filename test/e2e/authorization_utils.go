package e2e

import (
	"context"
	"strconv"
	"strings"

	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// testCase is a struct that represents a single testcase.
type testCase struct {
	data   testcaseData
	output testcaseOutput
}

// testcaseData is a struct that makes it user-friendly to write testcases
// more logically. This will be used to generate individual SubjectAccessReview
// objects in order to test the authorization rules.
type testcaseData struct {
	namespaces       []string
	names            []string
	verbs            []string
	apiGroups        []string
	resources        []string
	subresources     []string
	nonResourceVerbs []string
	nonResourcePaths []string
	users            []string

	// this is double slice since we need to check individually for
	// each group of users. A single slice would mean that the same user
	// is part of all the groups.
	groups [][]string
}

// testcaseOutput is a struct that represents the expected result of a testcase.
// This is based on the SubjectAccessReviewStatus type. It provides simplicity in testcase
// writing since one testcase can have multiple SubjectAccessReview objects
// and we need to determine the "final" result and expose that as the testcase output.
type testcaseOutput struct {
	// the final result based on results of individual SubjectAccessReview objects
	passed bool
	// the set of SARs that failed expectation in the test. This is an empty slice if the test passed.
	// It contains pretty printed SAR objects for debugging, when a test fails. It will
	// contain all the SAR objects that didn't match expectation.
	failingSARs []string
}

// String returns a pretty printed string of the testcase output. This is used
// to help debug the RBAC test cases.
func (o *testcaseOutput) String() string {
	outputStr := ""
	for _, sar := range o.failingSARs {
		outputStr += sar
	}
	return outputStr
}

func (t *testCase) run(ctx context.Context, cs kubernetes.Interface, allowExpected bool) error {
	// Generate the list of SubjectAccessReview objects based on the testcase data
	sars := t.generateSubjectAccessReviews()

	// Create the SubjectAccessReview objects in the cluster
	createdSars, err := createSubjectAccessReviews(ctx, cs, sars)
	if err != nil {
		return err
	}

	// Evaluate the output based on the created SubjectAccessReview objects
	// and set the final result in the testcase output
	t.evaluateOutput(createdSars, allowExpected)

	return nil
}

// accessReviewGenerator generates a list of SubjectAccessReview objects based on the
// testcase data provided.
func (t *testCase) generateSubjectAccessReviews() []authv1.SubjectAccessReview {
	// Initialize the list of SubjectAccessReview objects
	sars := make([]authv1.SubjectAccessReview, 0)

	// expand the testcaseData to generate a list of ResourceAttributes
	resourceAttributes := t.expandResourceAttributes()

	// expand the testcaseData to generate a list of NonResourceAttributes
	nonResourceAttributes := t.expandNonResourceAttributes()

	// expand users and groups
	userGroupExpansions := t.expandUsersAndGroups()

	// expand the ResourceAttributes, NonResourceAttributes and UserGroupExpansions
	// to generate a list of SubjectAccessReviewSpec objects
	sarSpecs := t.expandSubjectAccessReviewSpecs(resourceAttributes, nonResourceAttributes, userGroupExpansions)

	// create the SubjectAccessReview objects based on the SubjectAccessReviewSpec objects
	for _, sarSpec := range sarSpecs {
		sar := authv1.SubjectAccessReview{
			Spec: sarSpec,
		}
		sars = append(sars, sar)
	}
	return sars
}

// expandResourceAttributes expands the testcaseData to generate a list of ResourceAttributes
func (t *testCase) expandResourceAttributes() []authv1.ResourceAttributes {
	// This will hold the expanded ResourceAttributes
	ras := make([]authv1.ResourceAttributes, 0)

	nsExpansions := make([]authv1.ResourceAttributes, 0)
	// expand on namespaces
	if len(t.data.namespaces) > 0 {
		for _, ns := range t.data.namespaces {
			ra := authv1.ResourceAttributes{
				Namespace: ns,
			}
			nsExpansions = append(nsExpansions, ra)
		}
		// we update the expanded list with namespace expansions
		ras = nsExpansions
	}

	// expand on verbs
	verbExpansions := make([]authv1.ResourceAttributes, 0)
	if len(t.data.verbs) > 0 {
		for _, verb := range t.data.verbs {
			// If an expansion already take place, we need to copy and
			// change the existing objects
			if len(ras) > 0 {
				for _, ra := range ras {
					// copy the ResourceAttributes object to avoid modifying the original object
					// and make it safe to user in the next iterations
					copy := ra
					copy.Verb = verb
					verbExpansions = append(verbExpansions, copy)
				}
			} else {
				// If no expansion has taken place, we need to create a new object
				ra := authv1.ResourceAttributes{
					Verb: verb,
				}
				verbExpansions = append(verbExpansions, ra)
			}
		}
		// we update the expanded list with verb expansions
		ras = verbExpansions
	}

	// expand on apiGroups
	apiGroupExpansions := make([]authv1.ResourceAttributes, 0)
	if len(t.data.apiGroups) > 0 {
		for _, apiGroup := range t.data.apiGroups {
			if len(ras) > 0 {
				for _, ra := range ras {
					copy := ra
					copy.Group = apiGroup
					apiGroupExpansions = append(apiGroupExpansions, copy)
				}
			} else {
				ra := authv1.ResourceAttributes{
					Group: apiGroup,
				}
				apiGroupExpansions = append(apiGroupExpansions, ra)
			}
		}
		// we update the expanded list with apiGroup expansions
		ras = apiGroupExpansions
	}

	// expand on resources
	resourceExpansions := make([]authv1.ResourceAttributes, 0)
	if len(t.data.resources) > 0 {
		for _, resource := range t.data.resources {
			if len(ras) > 0 {
				for _, ra := range ras {
					copy := ra
					// split the resource string to get the group, resource and subresource
					parts := strings.Split(resource, "/")
					if len(parts) > 1 {
						switch len(parts) {
						case 2:
							copy.Group = parts[0]
							copy.Resource = parts[1]
						case 3:
							copy.Group = parts[0]
							copy.Resource = parts[1]
							copy.Subresource = parts[2]
						}
					} else {
						copy.Resource = parts[0]
					}
					resourceExpansions = append(resourceExpansions, copy)
				}
			} else {
				ra := authv1.ResourceAttributes{}
				// split the resource string to get the group, resource and subresource
				parts := strings.Split(resource, "/")
				if len(parts) > 1 {
					switch len(parts) {
					case 2:
						ra.Group = parts[0]
						ra.Resource = parts[1]
					case 3:
						ra.Group = parts[0]
						ra.Resource = parts[1]
						ra.Subresource = parts[2]
					}
				} else {
					ra.Resource = parts[0]
				}
				resourceExpansions = append(resourceExpansions, ra)
			}
		}
		// we update the expanded list with resource expansions
		ras = resourceExpansions
	}

	// expand on subresources
	subresourceExpansions := make([]authv1.ResourceAttributes, 0)
	if len(t.data.subresources) > 0 {
		for _, subresource := range t.data.subresources {
			if len(ras) > 0 {
				for _, ra := range ras {
					copy := ra
					copy.Subresource = subresource
					subresourceExpansions = append(subresourceExpansions, copy)
				}
			} else {
				ra := authv1.ResourceAttributes{
					Subresource: subresource,
				}
				subresourceExpansions = append(subresourceExpansions, ra)
			}
		}
		// we update the expanded list with subresource expansions
		ras = subresourceExpansions
	}

	// expand on names
	nameExpansions := make([]authv1.ResourceAttributes, 0)
	if len(t.data.names) > 0 {
		for _, name := range t.data.names {
			if len(ras) > 0 {
				for _, ra := range ras {
					copy := ra
					copy.Name = name
					nameExpansions = append(nameExpansions, copy)
				}
			} else {
				ra := authv1.ResourceAttributes{
					Name: name,
				}
				nameExpansions = append(nameExpansions, ra)
			}
		}
		// we update the expanded list with name expansions
		ras = nameExpansions
	}

	return ras
}

// expandNonResourceAttributes expands the testcaseData to generate a list of NonResourceAttributes
func (t *testCase) expandNonResourceAttributes() []authv1.NonResourceAttributes {
	// This will hold the expanded NonResourceAttributes
	nras := make([]authv1.NonResourceAttributes, 0)

	// expand on paths
	pathExpansions := make([]authv1.NonResourceAttributes, 0)
	if len(t.data.nonResourcePaths) > 0 {
		for _, path := range t.data.nonResourcePaths {
			nra := authv1.NonResourceAttributes{
				Path: path,
			}
			pathExpansions = append(pathExpansions, nra)
		}
		// we update the expanded list with path expansions
		nras = pathExpansions
	}

	// expand on verbs
	verbExpansions := make([]authv1.NonResourceAttributes, 0)
	if len(t.data.nonResourceVerbs) > 0 {
		for _, verb := range t.data.nonResourceVerbs {
			if len(nras) > 0 {
				for _, nra := range nras {
					copy := nra
					copy.Verb = verb
					verbExpansions = append(verbExpansions, copy)
				}
			} else {
				nra := authv1.NonResourceAttributes{
					Verb: verb,
				}
				verbExpansions = append(verbExpansions, nra)
			}
		}
		// we update the expanded list with verb expansions
		nras = verbExpansions
	}

	return nras
}

// expandUsersAndGroups expands the users and groups in the testcaseData to generate a list of SubjectAccessReviewSpecs
func (t *testCase) expandUsersAndGroups() []authv1.SubjectAccessReviewSpec {
	// This will hold the expanded SubjectAccessReviewSpec
	sars := make([]authv1.SubjectAccessReviewSpec, 0)

	// expand on users
	userExpansions := make([]authv1.SubjectAccessReviewSpec, 0)
	if len(t.data.users) > 0 {
		for _, user := range t.data.users {
			sar := authv1.SubjectAccessReviewSpec{
				User: user,
			}
			userExpansions = append(userExpansions, sar)
		}
		// we update the expanded list with user expansions
		sars = userExpansions
	}

	// expand on groups
	groupExpansions := make([]authv1.SubjectAccessReviewSpec, 0)
	if len(t.data.groups) > 0 {
		for _, group := range t.data.groups {
			// If an expansion already took place, we need to copy the
			// existing objects and change the groups
			if len(sars) > 0 {
				for _, sar := range sars {
					copy := sar
					copy.Groups = group
					groupExpansions = append(groupExpansions, copy)
				}
			} else {
				// If no expansion has taken place, we need to create a new object
				sar := authv1.SubjectAccessReviewSpec{
					Groups: group,
				}
				groupExpansions = append(groupExpansions, sar)
			}
		}
		// we update the expanded list with group expansions
		sars = groupExpansions
	}

	return sars
}

// expandSubjectAccessReviewSpecs takes the expanded ResourceAttributes, NonResourceAttributes and
// UserGroupExpansions and generates a list of SubjectAccessReviewSpec objects
func (t *testCase) expandSubjectAccessReviewSpecs(ras []authv1.ResourceAttributes, nras []authv1.NonResourceAttributes, userGroupExpansions []authv1.SubjectAccessReviewSpec) []authv1.SubjectAccessReviewSpec {
	sars := make([]authv1.SubjectAccessReviewSpec, 0)

	// expand on ResourceAttributes if they are defined
	rasExpansions := make([]authv1.SubjectAccessReviewSpec, 0)
	if len(ras) > 0 {
		for _, ra := range ras {
			for _, ug := range userGroupExpansions {
				sar := authv1.SubjectAccessReviewSpec{
					ResourceAttributes: &ra,
					User:               ug.User,
					Groups:             ug.Groups,
				}
				rasExpansions = append(rasExpansions, sar)
			}
		}
		// we update the expanded list with ResourceAttributes expansions
		sars = rasExpansions
	}

	// expand on NonResourceAttributes if they are defined
	nrasExpansions := make([]authv1.SubjectAccessReviewSpec, 0)
	if len(nras) > 0 {
		for _, nra := range nras {
			for _, ug := range userGroupExpansions {
				sar := authv1.SubjectAccessReviewSpec{
					NonResourceAttributes: &nra,
					User:                  ug.User,
					Groups:                ug.Groups,
				}
				nrasExpansions = append(nrasExpansions, sar)
			}
		}
		// we update the expanded list with NonResourceAttributes expansions
		sars = nrasExpansions
	}

	return sars
}

// createSubjectAccessReviews creates provided SubjectAccessReview objects in the cluster
func createSubjectAccessReviews(ctx context.Context, cs kubernetes.Interface, sars []authv1.SubjectAccessReview) ([]authv1.SubjectAccessReview, error) {
	createdSars := make([]authv1.SubjectAccessReview, 0)

	for _, sar := range sars {
		createdSar, err := createSubjectAccessReview(ctx, cs, sar)
		if err != nil {
			return createdSars, err
		}
		createdSars = append(createdSars, *createdSar)
	}
	return createdSars, nil
}

// createSubjectAccessReview creates a SubjectAccessReview object in the cluster
func createSubjectAccessReview(ctx context.Context, cs kubernetes.Interface, sar authv1.SubjectAccessReview) (*authv1.SubjectAccessReview, error) {
	return cs.AuthorizationV1().SubjectAccessReviews().Create(ctx, &sar, metav1.CreateOptions{})
}

// evaluateOutput evaluates the output based on the created SubjectAccessReview objects
// allowExpected is a boolean that determines if the expected result is 'allow' or 'deny'.
func (t *testCase) evaluateOutput(createdSars []authv1.SubjectAccessReview, allowExpected bool) {

	// Iterate over all the SubjectAccessReviews created and check for expecated result.
	// We don't break the loop if a result doesn't match expectation since we want to
	// capture all the failing SubjectAccessReviews for debugging.
	for _, sar := range createdSars {
		// if the expected result is 'allow' and the SAR is denied, we add it to the failingSARs
		if allowExpected && !sar.Status.Allowed {
			t.output.failingSARs = append(t.output.failingSARs, prettyPrintSAR(sar))
		}
		// if the expected result is 'deny' and the SAR is allowed, we add it to the failingSARs
		if !allowExpected && sar.Status.Allowed {
			t.output.failingSARs = append(t.output.failingSARs, prettyPrintSAR(sar))
		}
	}

	// if the failingSARs is empty, it means the test passed
	if len(t.output.failingSARs) == 0 {
		t.output.passed = true
	}
}

// prettyPrintSAR pretty prints the SubjectAccessReview object. This is used
// to help debug the RBAC test cases.
func prettyPrintSAR(sar authv1.SubjectAccessReview) string {

	str := "\nSubjectAccessReviewSpec:"
	// we print the field values conditionally since some fields might be empty
	// this helps in making the output more readable
	if ns := sar.Spec.ResourceAttributes.Namespace; ns != "" {
		str += "\n  Namespace: " + ns
	}
	if verb := sar.Spec.ResourceAttributes.Verb; verb != "" {
		str += "\n  Verb: " + verb
	}
	if group := sar.Spec.ResourceAttributes.Group; group != "" {
		str += "\n  APIGroup: " + group
	}
	if resource := sar.Spec.ResourceAttributes.Resource; resource != "" {
		str += "\n  Resource: " + resource
	}
	if subresource := sar.Spec.ResourceAttributes.Subresource; subresource != "" {
		str += "\n  Subresource: " + subresource
	}
	if name := sar.Spec.ResourceAttributes.Name; name != "" {
		str += "\n  Name: " + name
	}
	if sar.Spec.NonResourceAttributes != nil {
		if verb := sar.Spec.NonResourceAttributes.Verb; verb != "" {
			str += "\n  NonResourceVerb: " + verb
		}
		if path := sar.Spec.NonResourceAttributes.Path; path != "" {
			str += "\n  NonResourcePath: " + path
		}
	}
	if user := sar.Spec.User; user != "" {
		str += "\n  User: " + user
	}
	if groups := sar.Spec.Groups; len(groups) > 0 {
		str += "\n  Groups: " + strings.Join(groups, ",")
	}
	str += "\nSubjectAccessReviewStatus:"
	// these fields are always present in the SubjectAccessReviewStatus
	str += "\n  Allowed: " + strconv.FormatBool(sar.Status.Allowed)
	str += "\n  Denied: " + strconv.FormatBool(sar.Status.Denied)

	if reason := sar.Status.Reason; reason != "" {
		str += "\n  Reason: " + reason
	}
	str += "\n"
	return str
}
