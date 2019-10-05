package kube

import (
	v1Lister "github.com/equinor/radix-operator/pkg/client/listers/radix/v1"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	coreListers "k8s.io/client-go/listers/core/v1"
	extensionListers "k8s.io/client-go/listers/extensions/v1beta1"
	rbacListers "k8s.io/client-go/listers/rbac/v1"
)

// Radix Annotations
const (
	AdGroupsAnnotation    = "radix-app-adgroups"
	RadixBranchAnnotation = "radix-branch"

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
	kubeClient        kubernetes.Interface
	RdLister          v1Lister.RadixDeploymentLister
	NamespaceLister   coreListers.NamespaceLister
	SecretLister      coreListers.SecretLister
	DeploymentLister  extensionListers.DeploymentLister
	ServiceLister     coreListers.ServiceLister
	IngressLister     extensionListers.IngressLister
	RoleBindingLister rbacListers.RoleBindingLister
}

var logger *log.Entry

func init() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": "kube-api"})
}

// New Constructor
func New(client kubernetes.Interface) (*Kube, error) {
	kube := &Kube{
		kubeClient: client,
	}
	return kube, nil
}

// NewWithListers Constructor
func NewWithListers(client kubernetes.Interface,
	rdLister v1Lister.RadixDeploymentLister,
	namespaceLister coreListers.NamespaceLister,
	secretLister coreListers.SecretLister,
	deploymentLister extensionListers.DeploymentLister,
	serviceLister coreListers.ServiceLister,
	ingressLister extensionListers.IngressLister,
	roleBindingLister rbacListers.RoleBindingLister) (*Kube, error) {
	kube := &Kube{
		kubeClient:        client,
		RdLister:          rdLister,
		NamespaceLister:   namespaceLister,
		SecretLister:      secretLister,
		DeploymentLister:  deploymentLister,
		ServiceLister:     serviceLister,
		IngressLister:     ingressLister,
		RoleBindingLister: roleBindingLister,
	}
	return kube, nil
}

func isEmptyPatch(patchBytes []byte) bool {
	return string(patchBytes) == "{}"
}
