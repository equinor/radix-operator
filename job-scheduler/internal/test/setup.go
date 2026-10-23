package test

import (
	"fmt"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixclientfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"k8s.io/client-go/kubernetes"
	kubeclientfake "k8s.io/client-go/kubernetes/fake"
)

// SetupTest Setup test
func SetupTest(t *testing.T, appName, appEnvironment, appComponent, appDeployment string, historyLimit int) (radixclient.Interface, kubernetes.Interface, *kube.Kube) {
	t.Setenv("RADIX_APP", appName)
	t.Setenv("RADIX_ENVIRONMENT", appEnvironment)
	t.Setenv("RADIX_COMPONENT", appComponent)
	t.Setenv("RADIX_DEPLOYMENT", appDeployment)
	t.Setenv("RADIX_JOB_SCHEDULERS_PER_ENVIRONMENT_HISTORY_LIMIT", fmt.Sprint(historyLimit))
	kubeClient := kubeclientfake.NewSimpleClientset()
	radixClient := radixclientfake.NewSimpleClientset() // nolint:staticcheck // SA1019: Ignore linting deprecated fields
	kubeUtil, _ := kube.New(kubeClient, radixClient, nil, nil)
	return radixClient, kubeClient, kubeUtil
}

// Cleanup Cleanup test
func Cleanup(t *testing.T) {
	t.Cleanup(func() {})
}
