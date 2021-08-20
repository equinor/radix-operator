package kube

import (
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	v1Lister "github.com/equinor/radix-operator/pkg/client/listers/radix/v1"
	log "github.com/sirupsen/logrus"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	appsv1Listers "k8s.io/client-go/listers/apps/v1"
	batchListers "k8s.io/client-go/listers/batch/v1"
	coreListers "k8s.io/client-go/listers/core/v1"
	networkingListers "k8s.io/client-go/listers/networking/v1beta1"
	rbacListers "k8s.io/client-go/listers/rbac/v1"
)

// Radix Annotations
const (
	RadixBranchAnnotation          = "radix-branch"
	RadixComponentImagesAnnotation = "radix-component-images"
	RadixContainerOutputAnnotation = "radix-container-output"
	RadixDeploymentNameAnnotation  = "radix-deployment-name"

	// See https://github.com/equinor/radix-velero-plugin/blob/master/velero-plugins/deployment/restore.go
	RestoredStatusAnnotation = "equinor.com/velero-restored-status"
)

// Radix Labels
const (
	RadixAppLabel                = "radix-app"
	RadixEnvLabel                = "radix-env"
	RadixComponentLabel          = "radix-component"
	RadixComponentTypeLabel      = "radix-component-type"
	RadixJobNameLabel            = "radix-job-name"
	RadixBuildLabel              = "radix-build"
	RadixCommitLabel             = "radix-commit"
	RadixImageTagLabel           = "radix-image-tag"
	RadixJobTypeLabel            = "radix-job-type"
	RadixJobTypeJob              = "job" // Outer job
	RadixJobTypeBuild            = "build"
	RadixJobTypeScan             = "scan"
	RadixJobTypeCloneConfig      = "clone-config"
	RadixJobTypeJobSchedule      = "job-scheduler"
	RadixAppAliasLabel           = "radix-app-alias"
	RadixExternalAliasLabel      = "radix-app-external-alias"
	RadixActiveClusterAliasLabel = "radix-app-active-cluster-alias"
	RadixMountTypeLabel          = "mount-type"
	RadixVolumeMountNameLabel    = "radix-volume-mount-name"
	RadixGpuLabel                = "radix-node-gpu"
	RadixGpuCountLabel           = "radix-node-gpu-count"
	RadixNamespace               = "radix-namespace"
	RadixConfigMapTypeLabel      = "radix-config-map-type"

	// Only for backward compatibility
	RadixBranchDeprecated = "radix-branch"
)

// Kube  Stuct for accessing lower level kubernetes functions
type Kube struct {
	kubeClient               kubernetes.Interface
	radixclient              radixclient.Interface
	RrLister                 v1Lister.RadixRegistrationLister
	ReLister                 v1Lister.RadixEnvironmentLister
	RdLister                 v1Lister.RadixDeploymentLister
	NamespaceLister          coreListers.NamespaceLister
	ConfigMapLister          coreListers.ConfigMapLister
	SecretLister             coreListers.SecretLister
	DeploymentLister         appsv1Listers.DeploymentLister
	IngressLister            networkingListers.IngressLister
	ServiceLister            coreListers.ServiceLister
	RoleBindingLister        rbacListers.RoleBindingLister
	ClusterRoleBindingLister rbacListers.ClusterRoleBindingLister
	RoleLister               rbacListers.RoleLister
	ClusterRoleLister        rbacListers.ClusterRoleLister
	ServiceAccountLister     coreListers.ServiceAccountLister
	LimitRangeLister         coreListers.LimitRangeLister
	JobLister                batchListers.JobLister
}

var logger *log.Entry

func init() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": "kube-api"})
}

// New Constructor
func New(client kubernetes.Interface, radixClient radixclient.Interface) (*Kube, error) {
	kubeutil := &Kube{
		kubeClient:  client,
		radixclient: radixClient,
	}
	return kubeutil, nil
}

// NewWithListers Constructor
func NewWithListers(client kubernetes.Interface,
	radixclient radixclient.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	radixInformerFactory informers.SharedInformerFactory) (*Kube, error) {
	kubeutil := &Kube{
		kubeClient:               client,
		radixclient:              radixclient,
		RrLister:                 radixInformerFactory.Radix().V1().RadixRegistrations().Lister(),
		ReLister:                 radixInformerFactory.Radix().V1().RadixEnvironments().Lister(),
		RdLister:                 radixInformerFactory.Radix().V1().RadixDeployments().Lister(),
		NamespaceLister:          kubeInformerFactory.Core().V1().Namespaces().Lister(),
		ConfigMapLister:          kubeInformerFactory.Core().V1().ConfigMaps().Lister(),
		SecretLister:             kubeInformerFactory.Core().V1().Secrets().Lister(),
		DeploymentLister:         kubeInformerFactory.Apps().V1().Deployments().Lister(),
		ServiceLister:            kubeInformerFactory.Core().V1().Services().Lister(),
		IngressLister:            kubeInformerFactory.Networking().V1beta1().Ingresses().Lister(),
		RoleBindingLister:        kubeInformerFactory.Rbac().V1().RoleBindings().Lister(),
		ClusterRoleBindingLister: kubeInformerFactory.Rbac().V1().ClusterRoleBindings().Lister(),
		RoleLister:               kubeInformerFactory.Rbac().V1().Roles().Lister(),
		ClusterRoleLister:        kubeInformerFactory.Rbac().V1().ClusterRoles().Lister(),
		LimitRangeLister:         kubeInformerFactory.Core().V1().LimitRanges().Lister(),
		JobLister:                kubeInformerFactory.Batch().V1().Jobs().Lister(),
	}

	return kubeutil, nil
}

func IsEmptyPatch(patchBytes []byte) bool {
	return string(patchBytes) == "{}"
}
