package deployment

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RadixJobTypeLabel defines values for radix-job-type label
type RadixJobType string

// NewRadixJobTypeFromObjectLabels returns value of label radix-job-type from the object's labels
// ok returns false if label does not exist in object
func NewRadixJobTypeFromObjectLabels(object metav1.Object) (jobType RadixJobType, ok bool) {
	var jobTypeLabelValue string
	labels := object.GetLabels()

	jobTypeLabelValue, labelOk := labels[kube.RadixJobTypeLabel]
	if !labelOk {
		return "", false
	}

	return RadixJobType(jobTypeLabelValue), true
}

// IsJobScheduler checks if value of RadixJobType is job-scheduler (as defined in kube.RadixJobTypeJobSchedule)
func (t RadixJobType) IsJobScheduler() bool {
	return t == kube.RadixJobTypeJobSchedule
}
