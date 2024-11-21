package e2e

import (
	"context"
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
	allowed, denied bool
	// the set of reasons from individual SubjectAccessReview objects
	reason []string
}

func (t *testCase) run(ctx context.Context, cs kubernetes.Interface) error {
	// Generate the list of SubjectAccessReview objects based on the testcase data
	sars := t.generateSubjectAccessReviews()

	// Create the SubjectAccessReview objects in the cluster
	createdSars, err := createSubjectAccessReviews(ctx, cs, sars)
	if err != nil {
		return err
	}

	// Evaluate the output based on the created SubjectAccessReview objects
	// and set the final result in the testcase output
	t.evaluateOutput(createdSars)

	return nil
}

// accessReviewGenerator generates a list of SubjectAccessReview objects based on the
// testcase data provided.
func (t *testCase) generateSubjectAccessReviews() []authv1.SubjectAccessReview {
	// Initialize the list of SubjectAccessReview objects
	sars := make([]authv1.SubjectAccessReview, 0)

	// expand the testcase data to generate a list of ResourceAttributes
	resourceAttributes := t.expandResourceAttributes()

	// expand the testcase data to generate a list of NonResourceAttributes
	// nonResourceAttributes := t.expandNonResourceAttributes()

	// expand the testcase data to generate a list of SubjectAccessReview objects
	// based on the ResourceAttributes and NonResourceAttributes
	for _, ra := range resourceAttributes {
		for _, user := range t.data.users {
			for _, group := range t.data.groups {
				sar := authv1.SubjectAccessReview{
					Spec: authv1.SubjectAccessReviewSpec{
						ResourceAttributes: &ra,
						User:               user,
						Groups:             group,
					},
				}
				sars = append(sars, sar)
			}
		}
	}
	return sars
}

// expandResourceAttributes expands the testcase data to generate a list of ResourceAttributes
func (t *testCase) expandResourceAttributes() []authv1.ResourceAttributes {
	// This will hold the expanded ResourceAttributes
	ras := make([]authv1.ResourceAttributes, 0)

	// TODO: Convert this logic in a function similar to the way it is implemented
	// today to avoid code duplication
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

// expandNonResourceAttributes expands the testcase data to generate a list of NonResourceAttributes
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
			for _, nra := range nras {
				copy := nra
				copy.Verb = verb
				verbExpansions = append(verbExpansions, copy)
			}
		}
		// we update the expanded list with verb expansions
		nras = verbExpansions
	}

	return nras
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
func (t *testCase) evaluateOutput(createdSars []authv1.SubjectAccessReview) {
	tcOutput := testcaseOutput{}
	// TODO: Test should only pass if all SubjectAccessReviews have expected
	// value. Need to rethink this composition logic.
	// For example if we have 3 SubjectAccessReviews and the expecataion is 'deny',
	// then ALL 3 of them should have a 'denied: true' in response. In this implementation
	// even if 1 of them was denied, the test would pass even if the other 2 were allowed.

	// Iterate over all the SubjectAccessReviews created and determine the final result
	// We don't break the loop if we have denied access from one SubjectAccessReview,
	// we continue to check all of them and collect all reasons.
	for _, sar := range createdSars {
		// We skip the SubjectAccessReviews that are allowed
		if sar.Status.Allowed {
			continue
		}
		// If any of the SubjectAccessReviews have denied access, the final result is denied
		if sar.Status.Denied {
			tcOutput.denied = true
			tcOutput.reason = append(tcOutput.reason, sar.Status.Reason)
			continue
		}
		// Undecided access is also considered as denied
		if !sar.Status.Allowed && !sar.Status.Denied {
			tcOutput.denied = true
			tcOutput.reason = append(tcOutput.reason, sar.Status.Reason)
			continue
		}
	}

	// If none of the SubjectAccessReviews have denied access, the final result is allowed
	if !tcOutput.denied {
		tcOutput.allowed = true
	}

	t.output = tcOutput
}
