package deployment

import (
	certclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeploymentSyncerFactoryFunc is an adapter that can be used to convert
// a function into a DeploymentSyncerFactory
type DeploymentSyncerFactoryFunc func(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	dynamicClient client.Client,
	certClient certclient.Interface,
	registration *v1.RadixRegistration,
	radixDeployment *v1.RadixDeployment,
	ingressAnnotationProviders []ingress.AnnotationProvider,
	auxResourceManagers []AuxiliaryResourceManager,
	config *config.Config,
) DeploymentSyncer

func (f DeploymentSyncerFactoryFunc) CreateDeploymentSyncer(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	dynamicClient client.Client,
	certClient certclient.Interface,
	registration *v1.RadixRegistration,
	radixDeployment *v1.RadixDeployment,
	ingressAnnotationProviders []ingress.AnnotationProvider,
	auxResourceManagers []AuxiliaryResourceManager,
	config *config.Config,
) DeploymentSyncer {
	return f(kubeclient, kubeutil, radixclient, dynamicClient, certClient, registration, radixDeployment, ingressAnnotationProviders, auxResourceManagers, config)
}

// DeploymentSyncerFactory defines a factory to create a DeploymentSyncer
type DeploymentSyncerFactory interface {
	CreateDeploymentSyncer(
		kubeclient kubernetes.Interface,
		kubeutil *kube.Kube,
		radixclient radixclient.Interface,
		dynamicClient client.Client,
		certClient certclient.Interface,
		registration *v1.RadixRegistration,
		radixDeployment *v1.RadixDeployment,
		ingressAnnotationProviders []ingress.AnnotationProvider,
		auxResourceManagers []AuxiliaryResourceManager,
		config *config.Config,
	) DeploymentSyncer
}
