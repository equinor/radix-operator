package watcher

import (
	"context"
	"errors"
	"testing"
	"time"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	_ "github.com/equinor/radix-operator/pkg/apis/test"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

func setupTest(t *testing.T) (*radix.Clientset, *kubernetes.Clientset) {
	radixClient := radix.NewSimpleClientset()
	kubeClient := kubernetes.NewSimpleClientset()
	return radixClient, kubeClient
}

func TestDeploy_WaitActiveDeployment(t *testing.T) {
	const (
		namespace           = "app-dev"
		radixDeploymentName = "any-rd-name"
	)
	type scenario struct {
		name                  string
		hasRadixDevelopment   bool
		radixDeploymentStatus radixv1.RadixDeployStatus
		watcherError          error
	}
	scenarios := []scenario{
		{name: "Active deployment, no fail", hasRadixDevelopment: true, radixDeploymentStatus: radixv1.RadixDeployStatus{ActiveFrom: metav1.Time{Time: time.Now()}}, watcherError: nil},
		{name: "No active radix deployment, fail", hasRadixDevelopment: true, radixDeploymentStatus: radixv1.RadixDeployStatus{}, watcherError: errors.New("context deadline exceeded")},
		{name: "No radix deployment, fail", hasRadixDevelopment: false, watcherError: errors.New("radixdeployments.radix.equinor.com \"any-rd-name\" not found")},
	}
	for _, ts := range scenarios {
		t.Run(ts.name, func(tt *testing.T) {
			radixClient, kubeClient := setupTest(tt)
			require.NoError(t, createNamespace(kubeClient, namespace))

			if ts.hasRadixDevelopment {
				_, err := radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), &radixv1.RadixDeployment{
					ObjectMeta: metav1.ObjectMeta{Name: radixDeploymentName},
					Status:     ts.radixDeploymentStatus,
				}, metav1.CreateOptions{})
				require.NoError(tt, err)
			}

			watcher := NewRadixDeploymentWatcher(radixClient, time.Millisecond*10)
			err := watcher.WaitForActive(namespace, radixDeploymentName)

			if ts.watcherError == nil {
				assert.NoError(tt, err)
			} else {
				assert.EqualError(tt, err, ts.watcherError.Error())
			}
		})
	}
}

func createNamespace(kubeClient *kubernetes.Clientset, namespace string) error {
	_, err := kubeClient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{})
	return err
}
