package access

import (
	"context"

	authorizationapi "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// HasAccess checks if user has access to a resource
// cannot run as test - does not return correct values
func HasAccess(ctx context.Context, client kubernetes.Interface, resourceAttributes *authorizationapi.ResourceAttributes) (bool, error) {
	sar := authorizationapi.SelfSubjectAccessReview{
		Spec: authorizationapi.SelfSubjectAccessReviewSpec{
			ResourceAttributes: resourceAttributes,
		},
	}

	r, err := postSelfSubjectAccessReviews(ctx, client, sar)
	if err != nil {
		return false, err
	}
	return r.Status.Allowed, nil
}

func postSelfSubjectAccessReviews(ctx context.Context, client kubernetes.Interface, sar authorizationapi.SelfSubjectAccessReview) (*authorizationapi.SelfSubjectAccessReview, error) {
	return client.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, &sar, metav1.CreateOptions{})
}
