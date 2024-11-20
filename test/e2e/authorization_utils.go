package e2e

import (
	"context"

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
	sars, err := generateSubjectAccessReviews(t.data)
	if err != nil {
		return err
	}

	// Create the SubjectAccessReview objects in the cluster
	err = createSubjectAccessReviews(ctx, cs, sars)
	if err != nil {
		return err
	}

	// TODO: Implement the logic to determine the final result based on the results of individual SubjectAccessReview objects
	return nil
}

// accessReviewGenerator generates a list of SubjectAccessReview objects based on the
// testcase data provided.
func generateSubjectAccessReviews(data testcaseData) ([]authv1.SubjectAccessReview, error) {
	// TODO: Implement this function
	return nil, nil
}

// createSubjectAccessReviews creates provided SubjectAccessReview objects in the cluster
func createSubjectAccessReviews(ctx context.Context, cs kubernetes.Interface, sars []authv1.SubjectAccessReview) error {
	for _, sar := range sars {
		_, err := createSubjectAccessReview(ctx, cs, sar)
		if err != nil {
			return err
		}
	}
	return nil
}

// createSubjectAccessReview creates a SubjectAccessReview object in the cluster
func createSubjectAccessReview(ctx context.Context, cs kubernetes.Interface, sar authv1.SubjectAccessReview) (*authv1.SubjectAccessReview, error) {
	return cs.AuthorizationV1().SubjectAccessReviews().Create(ctx, &sar, metav1.CreateOptions{})
}
