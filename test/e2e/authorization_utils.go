package e2e

import (
	"context"
	"fmt"

	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func createSubjectAccessReview(ctx context.Context, cs kubernetes.Interface, sar authv1.SubjectAccessReview) (*authv1.SubjectAccessReview, error) {
	return cs.AuthorizationV1().SubjectAccessReviews().Create(ctx, &sar, metav1.CreateOptions{})
}

func createLocalClient() (kubernetes.Interface, error) {
	kubeconfigPath := "/workdir/test/e2e/kubeconfig"

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("unable to create a client: %v", err)
	}

	return client, nil
}

type testCase struct {
	name           string
	sar            authv1.SubjectAccessReview
	expectedStatus authv1.SubjectAccessReviewStatus
}
