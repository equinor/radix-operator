package job

import (
	"context"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetOwnerReference Gets owner reference of radix job
func GetOwnerReference(radixJob *v1.RadixJob) []metav1.OwnerReference {
	trueVar := true
	return []metav1.OwnerReference{
		{
			APIVersion: "radix.equinor.com/v1", //need to hardcode these values for now - seems they are missing from the CRD in k8s 1.8
			Kind:       "RadixJob",
			Name:       radixJob.Name,
			UID:        radixJob.UID,
			Controller: &trueVar,
		},
	}
}

// GetOwnerReferenceOfJob Gets owner reference of job with name and UUID
func GetOwnerReferenceOfJob(radixclient radixclient.Interface, namespace, name string) ([]metav1.OwnerReference, error) {
	job, err := radixclient.RadixV1().RadixJobs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return GetOwnerReference(job), nil
}
