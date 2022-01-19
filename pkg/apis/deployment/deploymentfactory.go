package deployment

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"k8s.io/client-go/kubernetes"
)

// DeploymentSyncerFactoryFunc is an adapter that can be used to convert
// a function into a DeploymentSyncerFactory
type DeploymentSyncerFactoryFunc func(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	prometheusperatorclient monitoring.Interface,
	registration *v1.RadixRegistration,
	radixDeployment *v1.RadixDeployment,
	forceRunAsNonRoot bool,
	tenantId string,
	oauth2DefaultConfig defaults.OAuth2DefaultConfig,
) DeploymentSyncer

func (f DeploymentSyncerFactoryFunc) CreateDeploymentSyncer(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	prometheusperatorclient monitoring.Interface,
	registration *v1.RadixRegistration,
	radixDeployment *v1.RadixDeployment,
	forceRunAsNonRoot bool,
	tenantId string,
	oauth2DefaultConfig defaults.OAuth2DefaultConfig,
) DeploymentSyncer {
	return f(kubeclient, kubeutil, radixclient, prometheusperatorclient, registration, radixDeployment, forceRunAsNonRoot, tenantId, oauth2DefaultConfig)
}

//DeploymentSyncerFactory defines a factory to create a DeploymentSyncer
type DeploymentSyncerFactory interface {
	CreateDeploymentSyncer(
		kubeclient kubernetes.Interface,
		kubeutil *kube.Kube,
		radixclient radixclient.Interface,
		prometheusperatorclient monitoring.Interface,
		registration *v1.RadixRegistration,
		radixDeployment *v1.RadixDeployment,
		forceRunAsNonRoot bool,
		tenantId string,
		oauth2DefaultConfig defaults.OAuth2DefaultConfig) DeploymentSyncer
}
