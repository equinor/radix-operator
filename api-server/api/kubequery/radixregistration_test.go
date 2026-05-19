package kubequery

import (
	"context"
	"testing"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_GetRadixRegistration(t *testing.T) {
	matched := radixv1.RadixRegistration{ObjectMeta: metav1.ObjectMeta{Name: "app1"}}
	client := radixfake.NewSimpleClientset(&matched) //nolint:staticcheck

	// Get existing RR
	actual, err := GetRadixRegistration(context.Background(), client, "app1")
	require.NoError(t, err)
	assert.Equal(t, &matched, actual)

	// Get non-existing RR
	_, err = GetRadixRegistration(context.Background(), client, "anyapp")
	assert.True(t, errors.IsNotFound(err))
}
