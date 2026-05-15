package kubequery

import (
	"context"
	"testing"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authorizationapiv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	testing2 "k8s.io/client-go/testing"
)

func Test_GetRadixApplication(t *testing.T) {
	matched := radixv1.RadixApplication{ObjectMeta: metav1.ObjectMeta{Name: "app1", Namespace: "app1-app"}}
	unmatched := radixv1.RadixApplication{ObjectMeta: metav1.ObjectMeta{Name: "app2", Namespace: "app2-any"}}
	client := radixfake.NewSimpleClientset(&matched, &unmatched) //nolint:staticcheck

	// Get existing RA
	actual, err := GetRadixApplication(context.Background(), client, "app1")
	require.NoError(t, err)
	assert.Equal(t, &matched, actual)

	// Get non-existing RA (wrong namespace)
	_, err = GetRadixApplication(context.Background(), client, "app2")
	assert.True(t, errors.IsNotFound(err))
}

func Test_IsRadixApplicationAdmin(t *testing.T) {
	called := 0
	client := fake.NewClientset()
	client.PrependReactor("create", "*", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
		createAction, ok := action.DeepCopy().(testing2.CreateAction)
		if !ok {
			return false, nil, nil
		}

		review, ok := createAction.GetObject().(*authorizationapiv1.SelfSubjectAccessReview)
		if !ok {
			return false, nil, nil
		}

		called++
		assert.Equal(t, review.Spec.ResourceAttributes, &authorizationapiv1.ResourceAttributes{
			Verb:     "patch",
			Group:    radixv1.GroupName,
			Resource: radixv1.ResourceRadixRegistrations,
			Version:  "*",
			Name:     "any-app-name",
		})
		return true, review, nil
	})

	actual, err := IsRadixApplicationAdmin(context.Background(), client, "any-app-name")
	require.NoError(t, err)
	assert.False(t, actual, "Should be false since we dont set it to true in our reactor")
	assert.Equal(t, 1, called)
}
