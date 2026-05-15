package kubequery

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func Test_GetEvents(t *testing.T) {
	matched := corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "event1", Namespace: "app1-dev"}}
	unmatched := corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "event2", Namespace: "app2-dev"}}
	client := kubefake.NewSimpleClientset(&matched, &unmatched) //nolint:staticcheck

	// Get existing events
	actual, err := GetEventsForEnvironment(context.Background(), client, "app1", "dev")
	require.NoError(t, err)
	assert.Len(t, actual, 1)
	assert.Equal(t, matched.GetName(), actual[0].GetName())

	// Get non-existing events (wrong namespace)
	actual, err = GetEventsForEnvironment(context.Background(), client, "app3", "dev")
	require.NoError(t, err)
	assert.Len(t, actual, 0)
}
