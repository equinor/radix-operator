package uncachedclient_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/equinor/radix-operator/pkg/apis/uncachedclient"
)

func TestNew_ReadsFromReaderAndWritesWithClient(t *testing.T) {
	ctx := context.Background()
	existing := &corev1.Secret{Name: "existing", Namespace: "ns"}
	// cached simulates a cache without the object, reader simulates the API server
	cached := fake.NewClientBuilder().Build()
	reader := fake.NewClientBuilder().WithObjects(existing).Build()

	c := uncachedclient.New(cached, reader)

	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(existing), &corev1.Secret{}))
	list := &corev1.SecretList{}
	require.NoError(t, c.List(ctx, list))
	assert.Len(t, list.Items, 1)

	created := &corev1.Secret{Name: "created", Namespace: "ns"}
	require.NoError(t, c.Create(ctx, created))
	require.NoError(t, cached.Get(ctx, client.ObjectKeyFromObject(created), &corev1.Secret{}))
	err := reader.Get(ctx, client.ObjectKeyFromObject(created), &corev1.Secret{})
	assert.True(t, kubeerrors.IsNotFound(err), "writes must go to the client, not the reader")
}
