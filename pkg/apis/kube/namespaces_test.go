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
	kubefake "k8s.io/client-go/kubernetes/fake"
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

	getAppNameFuncs := map[string]func(namespace *corev1.Namespace, name string) (string, error){
		"controller-runtime client": func(namespace *corev1.Namespace, name string) (string, error) {
			var objects []client.Object
			if namespace != nil {
				objects = append(objects, namespace)
			}
			reader := fake.NewClientBuilder().WithScheme(scheme.NewScheme()).WithObjects(objects...).Build()
			return kube.GetAppNameFromNamespace(context.Background(), name, reader)
		},
		"kube client": func(namespace *corev1.Namespace, name string) (string, error) {
			kubeClient := kubefake.NewClientset()
			if namespace != nil {
				if _, err := kubeClient.CoreV1().Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{}); err != nil {
					return "", err
				}
			}
			return kube.GetAppNameFromNamespaceWithKubeClient(context.Background(), name, kubeClient)
		},
	}

	for funcName, getAppName := range getAppNameFuncs {
		for name, tt := range tests {
			t.Run(funcName+"/"+name, func(t *testing.T) {
				appName, err := getAppName(tt.namespace, tt.name)
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
}
