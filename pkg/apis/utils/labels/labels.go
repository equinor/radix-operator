package labels

import (
	maputils "github.com/equinor/radix-common/utils/maps"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"
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

// ForComponentName returns labels describing the component name,
// e.g. "radix-component": "mycomponent"
func ForComponentName(componentName string) kubelabels.Set {
	return kubelabels.Set{
		kube.RadixComponentLabel: componentName,
	}
}

// ForComponentTypeName returns labels describing the component type,
// e.g. "radix-component-type": "job"
func ForComponentType(componentType v1.RadixComponentType) kubelabels.Set {
	return kubelabels.Set{
		kube.RadixComponentTypeLabel: string(componentType),
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

// ForServiceAccountIsForComponent returns labels indicating that a service account is used by a component or job
func ForServiceAccountIsForComponent() kubelabels.Set {
	return kubelabels.Set{
		kube.IsServiceAccountForComponent: "true",
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

func forAzureWorkloadUseIdentity() kubelabels.Set {
	return kubelabels.Set{
		azureWorkloadIdentityUseLabel: "true",
	}
}
