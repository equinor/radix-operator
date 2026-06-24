package labelselector

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	kubeLabels "k8s.io/apimachinery/pkg/labels"
)

// ForAuxiliaryResource returns a label set to be used as LabelSelector for auxiliary resource queries
func ForAuxiliaryResource(appName, componentName, auxType string) kubeLabels.Set {
	appSet := ForApplication(appName)
	appSet[kube.RadixAuxiliaryComponentLabel] = componentName
	appSet[kube.RadixAuxiliaryComponentTypeLabel] = auxType
	return appSet
}

// ForApplication returns label selector for Radix application
func ForApplication(appName string) kubeLabels.Set {
	return kubeLabels.Set{
		kube.RadixAppLabel: appName,
	}
}

// ForIsAppAlias returns label selector for "radix-app-alias"="true"
func ForIsAppAlias() kubeLabels.Set {
	return kubeLabels.Set{
		kube.RadixAppAliasLabel: "true",
	}
}

// ForComponent returns label selector for Radix application component
func ForComponent(appName, componentName string) kubeLabels.Set {
	return kubeLabels.Set{
		kube.RadixAppLabel:       appName,
		kube.RadixComponentLabel: componentName,
	}
}

// JobAndBatchJobsForComponent returns label selector for Radix application job-component jobs and batches
func JobAndBatchJobsForComponent(appName, componentName string) string {
	return fmt.Sprintf("%s=%s, %s=%s, %s in (%s, %s)", kube.RadixAppLabel, appName, kube.RadixComponentLabel, componentName, kube.RadixJobTypeLabel, kube.RadixJobTypeJobSchedule, kube.RadixJobTypeBatchSchedule)
}
