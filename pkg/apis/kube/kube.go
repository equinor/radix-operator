package kube

import (
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	v1Lister "github.com/equinor/radix-operator/pkg/client/listers/radix/v1"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	kedav1listers "github.com/kedacore/keda/v2/pkg/generated/listers/keda/v1alpha1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	appsv1Listers "k8s.io/client-go/listers/apps/v1"
	batchListers "k8s.io/client-go/listers/batch/v1"
	coreListers "k8s.io/client-go/listers/core/v1"
	networkingListers "k8s.io/client-go/listers/networking/v1"
	rbacListers "k8s.io/client-go/listers/rbac/v1"
	secretProviderClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
)

// Radix Annotations
const (
	RadixBranchAnnotation                            = "radix-branch"
	RadixGitTagsAnnotation                           = "radix.equinor.com/radix-git-tags"
	RadixCommitAnnotation                            = "radix.equinor.com/radix-commit"
	RadixConfigHash                                  = "radix.equinor.com/radix-config-hash"
	RadixBuildSecretHash                             = "radix.equinor.com/build-secret-hash"
	RadixComponentImagesAnnotation                   = "radix-component-images"
	RadixBuildComponentsAnnotation                   = "radix-build-component"
	RadixDeploymentNameAnnotation                    = "radix-deployment-name"
	RadixDeploymentPromotedFromDeploymentAnnotation  = "radix.equinor.com/radix-deployment-promoted-from-deployment"
	RadixDeploymentPromotedFromEnvironmentAnnotation = "radix.equinor.com/radix-deployment-promoted-from-environment"
	// See https://github.com/equinor/radix-velero-plugin/blob/master/velero-plugins/deployment/restore.go
	RestoredStatusAnnotation = "equinor.com/velero-restored-status"
)

// Radix Finalizers
const (
	RadixEnvironmentFinalizer = "radix.equinor.com/environment-finalizer"
	RadixDNSAliasFinalizer    = "radix.equinor.com/dnsalias-finalizer"
)

// Radix Labels
const (
	K8sAppLabel                         = "k8s-app"
	RadixAppLabel                       = "radix-app"
	RadixEnvLabel                       = "radix-env"
	RadixComponentLabel                 = "radix-component"
	RadixDeploymentLabel                = "radix-deployment"
	RadixComponentTypeLabel             = "radix-component-type"
	RadixJobNameLabel                   = "radix-job-name"
	RadixAuxiliaryComponentLabel        = "radix-aux-component"
	RadixAuxiliaryComponentTypeLabel    = "radix-aux-component-type"
	RadixBuildLabel                     = "radix-build"
	RadixCommitLabel                    = "radix-commit"
	RadixImageTagLabel                  = "radix-image-tag"
	RadixJobTypeLabel                   = "radix-job-type"
	RadixJobTypeJob                     = "job" // Outer job
	RadixJobTypeBuild                   = "build"
	RadixJobTypeCloneConfig             = "clone-config"
	RadixJobTypePreparePipelines        = "prepare-pipelines"
	RadixJobTypeRunPipelines            = "run-pipelines"
	RadixJobTypeJobSchedule             = "job-scheduler"
	RadixJobTypeBatchSchedule           = "batch-scheduler"
	RadixDefaultAliasLabel              = "radix-default-alias"
	RadixActiveClusterAliasLabel        = "radix-app-active-cluster-alias"
	RadixAppAliasLabel                  = "radix-app-alias"
	RadixExternalAliasLabel             = "radix-app-external-alias"
	RadixExternalAliasFQDNLabel         = "radix-app-external-alias-fqdn"
	RadixAliasLabel                     = "radix-alias"
	RadixMountTypeLabel                 = "mount-type"
	RadixVolumeMountNameLabel           = "radix-volume-mount-name"
	RadixGpuLabel                       = "radix-node-gpu"
	RadixGpuCountLabel                  = "radix-node-gpu-count"
	RadixJobNodeLabel                   = "nodepooltasks"
	RadixNamespace                      = "radix-namespace"
	RadixConfigMapTypeLabel             = "radix-config-map-type"
	RadixSecretTypeLabel                = "radix-secret-type"
	RadixSecretRefTypeLabel             = "radix-secret-ref-type"
	RadixSecretRefNameLabel             = "radix-secret-ref-name"
	RadixUserDefinedNetworkPolicyLabel  = "is-user-defined"
	RadixPodIsJobSchedulerLabel         = "is-job-scheduler-pod"
	RadixPodIsJobAuxObjectLabel         = "is-job-aux-object"
	IsServiceAccountForComponent        = "is-service-account-for-component"
	IsServiceAccountForSubPipelineLabel = "is-service-account-for-subpipeline"
	RadixBatchNameLabel                 = "radix-batch-name"
	RadixBatchJobNameLabel              = "radix-batch-job-name"
	RadixBatchTypeLabel                 = "radix-batch-type"
	RadixAccessValidationLabel          = "radix-access-validation"
	RadixPipelineTypeLabels             = "radix-pipeline"

	// NodeTaintGpuCountKey defines the taint key on GPU nodes.
	// Pods required to run on nodes with this taint must add a toleration with effect NoSchedule
	NodeTaintGpuCountKey = "radix-node-gpu-count"
	NodeTaintJobsKey     = "nodepooltasks"
)

// RadixBatchType defines value for use with label RadixBatchTypeLabel
type RadixBatchType string

const (
	RadixBatchTypeJob   RadixBatchType = "job"
	RadixBatchTypeBatch RadixBatchType = "batch"
)

// RadixSecretType defines value for use with label RadixSecretTypeLabel
type RadixSecretType string

const (
	// RadixSecretJobPayload Used by radix-job-scheduler to label secrets with payloads
	RadixSecretJobPayload RadixSecretType = "scheduler-job-payload"
)

// RadixConfigMapType Purpose of ConfigMap
type RadixConfigMapType string

const (
	// EnvVarsConfigMap ConfigMap contains environment variables
	EnvVarsConfigMap RadixConfigMapType = "env-vars"
	// EnvVarsMetadataConfigMap ConfigMap contains environment variables metadata
	EnvVarsMetadataConfigMap RadixConfigMapType = "env-vars-metadata"
	// RadixPipelineResultConfigMap Label of a ConfigMap, which keeps a Radix pipeline result
	RadixPipelineResultConfigMap RadixConfigMapType = "radix-pipeline-result"
)

// Kube  Struct for accessing lower level kubernetes functions
type Kube struct {
	kubeClient               kubernetes.Interface
	radixclient              radixclient.Interface
	kedaClient               kedav2.Interface
	secretProviderClient     secretProviderClient.Interface
	RrLister                 v1Lister.RadixRegistrationLister
	ReLister                 v1Lister.RadixEnvironmentLister
	RdLister                 v1Lister.RadixDeploymentLister
	RbLister                 v1Lister.RadixBatchLister
	RadixAlertLister         v1Lister.RadixAlertLister
	RadixDNSAliasLister      v1Lister.RadixDNSAliasLister
	NamespaceLister          coreListers.NamespaceLister
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
	ScaledObjectLister       kedav1listers.ScaledObjectLister

	// Do not use ConfigMapLister as it were cases it return outdated data
}

// New Constructor
func New(client kubernetes.Interface, radixClient radixclient.Interface, kedaClient kedav2.Interface, secretProviderClient secretProviderClient.Interface) (*Kube, error) {
	kubeutil := &Kube{
		kubeClient:           client,
		radixclient:          radixClient,
		kedaClient:           kedaClient,
		secretProviderClient: secretProviderClient,
	}
	return kubeutil, nil
}

// NewWithListers Constructor
func NewWithListers(client kubernetes.Interface,
	radixclient radixclient.Interface, kedaClient kedav2.Interface,
	secretProviderClient secretProviderClient.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	radixInformerFactory informers.SharedInformerFactory) (*Kube, error) {
	kubeutil := &Kube{
		kubeClient:               client,
		radixclient:              radixclient,
		kedaClient:               kedaClient,
		secretProviderClient:     secretProviderClient,
		RrLister:                 radixInformerFactory.Radix().V1().RadixRegistrations().Lister(),
		ReLister:                 radixInformerFactory.Radix().V1().RadixEnvironments().Lister(),
		RdLister:                 radixInformerFactory.Radix().V1().RadixDeployments().Lister(),
		RbLister:                 radixInformerFactory.Radix().V1().RadixBatches().Lister(),
		RadixAlertLister:         radixInformerFactory.Radix().V1().RadixAlerts().Lister(),
		RadixDNSAliasLister:      radixInformerFactory.Radix().V1().RadixDNSAliases().Lister(),
		NamespaceLister:          kubeInformerFactory.Core().V1().Namespaces().Lister(),
		SecretLister:             kubeInformerFactory.Core().V1().Secrets().Lister(),
		DeploymentLister:         kubeInformerFactory.Apps().V1().Deployments().Lister(),
		ServiceLister:            kubeInformerFactory.Core().V1().Services().Lister(),
		IngressLister:            kubeInformerFactory.Networking().V1().Ingresses().Lister(),
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

// KubeClient Kubernetes client
func (kubeutil *Kube) KubeClient() kubernetes.Interface {
	return kubeutil.kubeClient
}

// RadixClient Radix Kubernetes CRD client
func (kubeutil *Kube) RadixClient() radixclient.Interface {
	return kubeutil.radixclient
}
