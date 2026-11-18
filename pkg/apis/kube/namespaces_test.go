package kube_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/scheme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetAppNameFromNamespace(t *testing.T) {
	tests := map[string]struct {
		namespace *corev1.Namespace
		name      string
		wantName  string
		wantError string
	}{
		"returns the app label": {
			namespace: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
				Name:   "app-dev",
				Labels: map[string]string{kube.RadixAppLabel: "app"},
			}},
			name:     "app-dev",
			wantName: "app",
		},
		"errors when the app label is empty": {
			namespace: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-dev"}},
			name:      "app-dev",
			wantError: kube.RadixAppLabel,
		},
		"errors when the namespace does not exist": {
			name:      "missing",
			wantError: "failed to get namespace missing",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var objects []client.Object
			if tt.namespace != nil {
				objects = append(objects, tt.namespace)
			}
			reader := fake.NewClientBuilder().WithScheme(scheme.NewScheme()).WithObjects(objects...).Build()

			appName, err := kube.GetAppNameFromNamespace(context.Background(), tt.name, reader)
			if tt.wantError != "" {
				assert.ErrorContains(t, err, tt.wantError)
				assert.Empty(t, appName)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, appName)
		})
	}
}
