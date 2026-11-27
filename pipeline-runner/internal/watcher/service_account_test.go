package watcher

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

func TestServiceAccountWatcher_WaitFor(t *testing.T) {
	const (
		namespace          = "anyapp-app"
		serviceAccountName = "subpipeline-dev-sa"
	)
	type scenario struct {
		name                      string
		existingServiceAccounts   []corev1.ServiceAccount
		createServiceAccountAfter time.Duration
		waitTimeout               time.Duration
		cancelContext             bool
		expectedError             string
	}
	scenarios := []scenario{
		{
			name:                    "Service account exists, no fail",
			existingServiceAccounts: []corev1.ServiceAccount{{Name: serviceAccountName, Namespace: namespace}},
			waitTimeout:             5 * time.Second,
		},
		{
			name:                      "Service account created while waiting, no fail",
			createServiceAccountAfter: 1500 * time.Millisecond,
			waitTimeout:               10 * time.Second,
		},
		{
			name:          "Service account never created, fail with timeout",
			waitTimeout:   1500 * time.Millisecond,
			expectedError: "service account subpipeline-dev-sa in namespace anyapp-app was not created within 1.5s",
		},
		{
			name:                    "Service account exists in another namespace, fail with timeout",
			existingServiceAccounts: []corev1.ServiceAccount{{Name: serviceAccountName, Namespace: "otherapp-app"}},
			waitTimeout:             1500 * time.Millisecond,
			expectedError:           "service account subpipeline-dev-sa in namespace anyapp-app was not created within 1.5s",
		},
		{
			name:          "Parent context cancelled, fail",
			waitTimeout:   time.Minute,
			cancelContext: true,
			expectedError: "failed to wait for service account subpipeline-dev-sa in namespace anyapp-app: context canceled",
		},
	}
	for _, ts := range scenarios {
		t.Run(ts.name, func(tt *testing.T) {
			kubeClient := kubernetes.NewSimpleClientset()
			for _, sa := range ts.existingServiceAccounts {
				_, err := kubeClient.CoreV1().ServiceAccounts(sa.Namespace).Create(context.Background(), &sa, metav1.CreateOptions{})
				require.NoError(tt, err)
			}
			if ts.createServiceAccountAfter > 0 {
				go func() {
					time.Sleep(ts.createServiceAccountAfter)
					_, _ = kubeClient.CoreV1().ServiceAccounts(namespace).Create(context.Background(), &corev1.ServiceAccount{Name: serviceAccountName, Namespace: namespace}, metav1.CreateOptions{})
				}()
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if ts.cancelContext {
				cancel()
			}

			watcher := NewServiceAccountWatcher(kubeClient, ts.waitTimeout)
			err := watcher.WaitFor(ctx, namespace, serviceAccountName)

			if ts.expectedError == "" {
				assert.NoError(tt, err)
			} else {
				assert.EqualError(tt, err, ts.expectedError)
			}
		})
	}
}
