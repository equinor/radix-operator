package scheduledjob

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *syncer) buildJobSpec() (*batchv1.JobSpec, error) {
	rd, err := s.getRadixDeployment()
	if err != nil {
		return nil, err
	}
	jobComponent := rd.GetComponentByName(s.radixScheduledJob.Spec.RadixDeploymentJobRef.Job)
	if jobComponent == nil {
		return nil, fmt.Errorf("radix deployment %s does not contain a job with name %s", rd.GetName(), s.radixScheduledJob.Spec.RadixDeploymentJobRef.Job)
	}

	labels := radixlabels.Merge(
		radixlabels.ForApplicationName(rd.Spec.AppName),
		radixlabels.ForComponentName(jobComponent.Name),
		radixlabels.ForPodWithRadixIdentity(jobComponent.Identity),
		map[string]string{kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule},
	)
	serviceAccountSpec := deployment.NewServiceAccountSpec(rd, jobComponent)

	jobSpec := &batchv1.JobSpec{
		BackoffLimit: numbers.Int32Ptr(0),
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: corev1.PodSpec{
				RestartPolicy:                corev1.RestartPolicyNever,
				ServiceAccountName:           serviceAccountSpec.ServiceAccountName(),
				AutomountServiceAccountToken: serviceAccountSpec.AutomountServiceAccountToken(),
			},
		},
	}
	return jobSpec, nil
}
