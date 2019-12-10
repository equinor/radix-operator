package kube

import (
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	v1Lister "github.com/equinor/radix-operator/pkg/client/listers/radix/v1"
	log "github.com/sirupsen/logrus"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	coreListers "k8s.io/client-go/listers/core/v1"
	extensionListers "k8s.io/client-go/listers/extensions/v1beta1"
)

// Radix Annotations
const (
	RadixBranchAnnotation          = "radix-branch"
	RadixComponentImagesAnnotation = "radix-component-images"

	// See https://github.com/equinor/radix-velero-plugin/blob/master/velero-plugins/deployment/restore.go
	RestoredStatusAnnotation = "equinor.com/velero-restored-status"
)

// Radix Labels
const (
	RadixAppLabel                = "radix-app"
	RadixEnvLabel                = "radix-env"
	RadixComponentLabel          = "radix-component"
	RadixJobNameLabel            = "radix-job-name"
	RadixBuildLabel              = "radix-build"
	RadixCommitLabel             = "radix-commit"
	RadixImageTagLabel           = "radix-image-tag"
	RadixJobTypeLabel            = "radix-job-type"
	RadixJobTypeBuild            = "build"
	RadixAppAliasLabel           = "radix-app-alias"
	RadixExternalAliasLabel      = "radix-app-external-alias"
	RadixActiveClusterAliasLabel = "radix-app-active-cluster-alias"

	// Only for backward compatibility
	RadixBranchDeprecated = "radix-branch"
)

// Kube  Stuct for accessing lower level kubernetes functions
type Kube struct {
	kubeClient      kubernetes.Interface
	radixclient     radixclient.Interface
	RrLister        v1Lister.RadixRegistrationLister
	RdLister        v1Lister.RadixDeploymentLister
	NamespaceLister coreListers.NamespaceLister
	IngressLister   extensionListers.IngressLister
}

var logger *log.Entry

func init() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": "kube-api"})
}

// New Constructor
func New(client kubernetes.Interface, radixClient radixclient.Interface) (*Kube, error) {
	kube := &Kube{
		kubeClient:  client,
		radixclient: radixClient,
	}
	return kube, nil
}

// NewWithListers Constructor
func NewWithListers(client kubernetes.Interface,
	radixclient radixclient.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	radixInformerFactory informers.SharedInformerFactory) (*Kube, error) {
	kube := &Kube{
		kubeClient:      client,
		radixclient:     radixclient,
		RrLister:        radixInformerFactory.Radix().V1().RadixRegistrations().Lister(),
		RdLister:        radixInformerFactory.Radix().V1().RadixDeployments().Lister(),
		NamespaceLister: kubeInformerFactory.Core().V1().Namespaces().Lister(),
		IngressLister:   kubeInformerFactory.Extensions().V1beta1().Ingresses().Lister(),
	}

	return kube, nil
}

func isEmptyPatch(patchBytes []byte) bool {
	return string(patchBytes) == "{}"
}
