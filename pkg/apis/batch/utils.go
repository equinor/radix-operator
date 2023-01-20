package batch

import (
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"
)

func isResourceLabeledWithBatchJobName(batchJobName string, resource metav1.Object) bool {
	return kubelabels.SelectorFromSet(radixlabels.ForBatchJobName(batchJobName)).Matches(kubelabels.Set(resource.GetLabels()))
}

func isBatchJobStopRequested(batchJob radixv1.RadixBatchJob) bool {
	return batchJob.Stop != nil && *batchJob.Stop
}

func getKubeServiceName(batchName, batchJobName string) string {
	return fmt.Sprintf("%s-%s", batchName, batchJobName)
}

func getKubeJobName(batchName, batchJobName string) string {
	return fmt.Sprintf("%s-%s", batchName, batchJobName)
}
