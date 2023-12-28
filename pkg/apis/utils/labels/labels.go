package labels

import (
	maputils "github.com/equinor/radix-common/utils/maps"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

const azureWorkloadIdentityUseLabel = "azure.workload.identity/use"

// Merge multiple maps into one
func Merge(labels ...map[string]string) kubelabels.Set {
	return maputils.MergeMaps(labels...)
}

// ForApplicationName returns labels describing the application name,
// e.g. "radix-app": "myapp"
func ForApplicationName(appName string) kubelabels.Set {
	return kubelabels.Set{
		kube.RadixAppLabel: appName,
	}
}

// ForEnvironmentName returns labels describing the application environment name,
// e.g. "radix-env": "dev"
func ForEnvironmentName(envName string) kubelabels.Set {
	return kubelabels.Set{
		kube.RadixEnvLabel: envName,
	}
}

// ForComponentName returns labels describing the component name,
// e.g. "radix-component": "mycomponent"
func ForComponentName(componentName string) kubelabels.Set {
	return kubelabels.Set{
		kube.RadixComponentLabel: componentName,
	}
}

// ForJobAuxObject returns labels describing the job aux object,
func ForJobAuxObject(jobName string) kubelabels.Set {
	return kubelabels.Set{
		kube.RadixComponentLabel:         jobName,
		kube.RadixPodIsJobAuxObjectLabel: "true",
	}
}

// ForComponentType returns labels describing the component type,
// e.g. "radix-component-type": "job"
func ForComponentType(componentType v1.RadixComponentType) kubelabels.Set {
	return kubelabels.Set{
		kube.RadixComponentTypeLabel: string(componentType),
	}
}

// ForBatchType returns labels describing the type of batch,
// e.g. "radix-batch-type": "batch"
func ForBatchType(batchType kube.RadixBatchType) kubelabels.Set {
	return kubelabels.Set{
		kube.RadixBatchTypeLabel: string(batchType),
	}
}

// ForBatchName returns labels describing name of a batch,
// e.g. "radix-batch-name": "compute-20221212125307-pet6fubk"
func ForBatchName(batchName string) kubelabels.Set {
	return kubelabels.Set{
		kube.RadixBatchNameLabel: batchName,
	}
}

// ForBatchJobName returns labels describing name of job in a batch,
// e.g. "radix-batch-job-name": "fns63hk8"
func ForBatchJobName(jobName string) kubelabels.Set {
	return kubelabels.Set{
		kube.RadixBatchJobNameLabel: jobName,
	}
}

// ForCommitId returns labels describing the commit ID,
// e.g. "radix-commit": "64b54f4a6aa542cb4bd15666c1e104eee647573a"
func ForCommitId(commitId string) kubelabels.Set {
	return kubelabels.Set{
		kube.RadixCommitLabel: commitId,
	}
}

// ForPodIsJobScheduler returns labels indicating that a pod is a job scheduler,
func ForPodIsJobScheduler() kubelabels.Set {
	return kubelabels.Set{
		kube.RadixPodIsJobSchedulerLabel: "true",
	}
}

// ForIsJobAuxObject returns labels indicating that an object is a job auxiliary object,
func ForIsJobAuxObject() kubelabels.Set {
	return kubelabels.Set{
		kube.RadixPodIsJobAuxObjectLabel: "true",
	}
}

// ForServiceAccountIsForComponent returns labels indicating that a service account is used by a component or job
func ForServiceAccountIsForComponent() kubelabels.Set {
	return kubelabels.Set{
		kube.IsServiceAccountForComponent: "true",
	}
} // ForServiceAccountIsForComponent returns labels indicating that a service account is used by a component or job
func ForServiceAccountIsForSubPipeline() kubelabels.Set {
	return kubelabels.Set{
		kube.IsServiceAccountForSubPipelineLabel: "true",
	}
}

// ForServiceAccountWithRadixIdentity returns labels for configuring a ServiceAccount with external identities,
// e.g. for Azure Workload Identity: "azure.workload.identity/use": "true"
func ForServiceAccountWithRadixIdentity(identity *v1.Identity) kubelabels.Set {
	if identity == nil {
		return nil
	}

	var labels kubelabels.Set

	if identity.Azure != nil {
		labels = Merge(labels, forAzureWorkloadUseIdentity())
	}

	return labels
}

// ForPodWithRadixIdentity returns labels for configuring a Pod with external identities,
// e.g. for Azure Workload Identity: "azure.workload.identity/use": "true"
func ForPodWithRadixIdentity(identity *v1.Identity) kubelabels.Set {
	if identity == nil {
		return nil
	}

	var labels kubelabels.Set

	if identity.Azure != nil {
		labels = Merge(labels, forAzureWorkloadUseIdentity())
	}

	return labels
}

// ForJobType returns labels describing the job type,
// e.g. "radix-job-type": "batch-scheduler"
func ForJobType(jobType string) kubelabels.Set {
	return kubelabels.Set{
		kube.RadixJobTypeLabel: jobType,
	}
}

// ForBatchScheduleJobType returns labels describing the batch schedule job type
func ForBatchScheduleJobType() kubelabels.Set {
	return ForJobType(kube.RadixJobTypeBatchSchedule)
}

// ForJobScheduleJobType returns labels describing the job schedule job type
func ForJobScheduleJobType() kubelabels.Set {
	return ForJobType(kube.RadixJobTypeJobSchedule)
}

// ForPipelineJobName returns labels describing the name of a pipeline job
func ForPipelineJobName(jobName string) kubelabels.Set {
	return kubelabels.Set{
		kube.RadixJobNameLabel: jobName,
	}
}

// ForPipelineJobType returns labels describing the pipeline-job type
func ForPipelineJobType() kubelabels.Set {
	return ForJobType(kube.RadixJobTypeJob)
}

// ForPipelineJobPipelineType returns label describing the pipeline-job pipeline type, e.g. build-deploy, promote, deploy-only
func ForPipelineJobPipelineType(pipeline v1.RadixPipelineType) kubelabels.Set {
	return kubelabels.Set{
		kube.RadixPipelineTypeLabels: string(pipeline),
	}
}

// ForRadixImageTag returns labels describing that image tag used by pipeline job in a build-deploy scenario
func ForRadixImageTag(imageTag string) kubelabels.Set {
	return kubelabels.Set{
		kube.RadixImageTagLabel: imageTag,
	}
}

func forAzureWorkloadUseIdentity() kubelabels.Set {
	return kubelabels.Set{
		azureWorkloadIdentityUseLabel: "true",
	}
}

// GetRadixBatchDescendantsSelector returns selector for radix batch descendants - jobs, secrets, etc.
func GetRadixBatchDescendantsSelector(componentName string) kubelabels.Selector {
	return kubelabels.SelectorFromSet(Merge(ForJobScheduleJobType(), ForComponentName(componentName))).
		Add(*requirementRadixBatchNameLabelExists())
}

func requirementRadixBatchNameLabelExists() *kubelabels.Requirement {
	requirement, err := kubelabels.NewRequirement(kube.RadixBatchNameLabel, selection.Exists, []string{})
	if err != nil {
		panic(err)
	}
	return requirement
}

// ForRadixSecretType returns labels describing the radix secret type,
// e.g. "radix-secret-type": "scheduler-job-payload"
func ForRadixSecretType(secretType kube.RadixSecretType) kubelabels.Set {
	return kubelabels.Set{
		kube.RadixSecretTypeLabel: string(secretType),
	}
}

// ForAccessValidation returns labels indicating that an object is used for access validation
func ForAccessValidation() kubelabels.Set {
	return kubelabels.Set{
		kube.RadixAccessValidationLabel: "true",
	}
}

// ForAuxComponent returns labels for application component aux OAuth proxy
func ForAuxComponent(appName string, component v1.RadixCommonDeployComponent) map[string]string {
	return map[string]string{
		kube.RadixAppLabel:                    appName,
		kube.RadixAuxiliaryComponentLabel:     component.GetName(),
		kube.RadixAuxiliaryComponentTypeLabel: defaults.OAuthProxyAuxiliaryComponentType,
	}
}

// ForAuxComponentDefaultIngress returns labels for application component aux OAuth proxy default ingress
func ForAuxComponentDefaultIngress(appName string, component v1.RadixCommonDeployComponent) kubelabels.Set {
	return forAuxComponentIngress(appName, component, kube.RadixDefaultAliasLabel, "true")
}

// ForAuxComponentExternalAliasIngress returns labels for application component aux OAuth proxy external alias ingress
func ForAuxComponentExternalAliasIngress(appName string, component v1.RadixCommonDeployComponent) kubelabels.Set {
	return forAuxComponentIngress(appName, component, kube.RadixExternalAliasLabel, "true")
}

// ForAuxComponentActiveClusterAliasIngress returns labels for application component active cluster alias ingress
func ForAuxComponentActiveClusterAliasIngress(appName string, component v1.RadixCommonDeployComponent) kubelabels.Set {
	return forAuxComponentIngress(appName, component, kube.RadixActiveClusterAliasLabel, "true")
}

// ForAuxComponentDNSAliasIngress returns labels for application component aux DNS alias ingress OAuth proxy
func ForAuxComponentDNSAliasIngress(appName string, component v1.RadixCommonDeployComponent, dnsAlias string) kubelabels.Set {
	return forAuxComponentIngress(appName, component, kube.RadixAliasLabel, dnsAlias)
}

func forAuxComponentIngress(appName string, component v1.RadixCommonDeployComponent, aliasLabel, aliasLabelValue string) kubelabels.Set {
	return kubelabels.Set{
		kube.RadixAppLabel:                    appName,
		kube.RadixAuxiliaryComponentLabel:     component.GetName(),
		kube.RadixAuxiliaryComponentTypeLabel: defaults.OAuthProxyAuxiliaryComponentType,
		aliasLabel:                            aliasLabelValue,
	}
}

// ForDNSAliasIngress returns labels for DNS alias ingress
func ForDNSAliasIngress(appName string, componentName, dnsAlias string) kubelabels.Set {
	return kubelabels.Set{
		kube.RadixAppLabel:       appName,
		kube.RadixComponentLabel: componentName,
		kube.RadixAliasLabel:     dnsAlias,
	}
}

// ForComponentExternalAliasIngress returns labels for application component external alias ingress
func ForComponentExternalAliasIngress(component v1.RadixCommonDeployComponent) kubelabels.Set {
	return kubelabels.Set{
		kube.RadixComponentLabel:     component.GetName(),
		kube.RadixExternalAliasLabel: "true",
	}
}

// ForComponentDefaultAliasIngress returns labels for application component default alias ingress
func ForComponentDefaultAliasIngress(component v1.RadixCommonDeployComponent) kubelabels.Set {
	return kubelabels.Set{
		kube.RadixComponentLabel:    component.GetName(),
		kube.RadixDefaultAliasLabel: "true",
	}
}

// ForComponentActiveClusterAliasIngress returns labels for application component active cluster alias ingress
func ForComponentActiveClusterAliasIngress(component v1.RadixCommonDeployComponent) kubelabels.Set {
	return kubelabels.Set{
		kube.RadixComponentLabel:          component.GetName(),
		kube.RadixActiveClusterAliasLabel: "true",
	}
}

// ForDNSAliasRbac returns labels for DNS alias cluster role and rolebinding
func ForDNSAliasRbac(appName string) kubelabels.Set {
	return kubelabels.Set{
		kube.RadixAppLabel:   appName,
		kube.RadixAliasLabel: "true",
	}
}
