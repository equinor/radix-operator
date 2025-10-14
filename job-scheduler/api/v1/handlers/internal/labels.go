package internal

import (
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	kubeLabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

// GetLabelSelectorForAllRadixBatchesPods Gets a label selector for all radix batches pods
func GetLabelSelectorForAllRadixBatchesPods(componentName string) string {
	radixBatchJobNameExistsReq, _ := kubeLabels.NewRequirement(kube.RadixBatchJobNameLabel, selection.Exists, []string{})
	radixBatchJobNameExistsSel := kubeLabels.NewSelector().Add(*radixBatchJobNameExistsReq)
	return strings.Join([]string{kubeLabels.SelectorFromSet(
		labels.Merge(
			labels.ForComponentName(componentName),
			labels.ForJobScheduleJobType(),
		)).String(),
		radixBatchJobNameExistsSel.String()},
		",")
}

// GetLabelSelectorForRadixBatchesPods Gets a label selector for a radix batch pods
func GetLabelSelectorForRadixBatchesPods(componentName, batchName string) string {
	radixBatchJobNameExistsReq, _ := kubeLabels.NewRequirement(kube.RadixBatchJobNameLabel, selection.Exists, []string{})
	radixBatchJobNameExistsSel := kubeLabels.NewSelector().Add(*radixBatchJobNameExistsReq)
	return strings.Join([]string{kubeLabels.SelectorFromSet(
		labels.Merge(
			labels.ForComponentName(componentName),
			labels.ForBatchName(batchName),
			labels.ForJobScheduleJobType(),
		)).String(),
		radixBatchJobNameExistsSel.String()},
		",")
}
