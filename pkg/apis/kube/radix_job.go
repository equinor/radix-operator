package kube

import (
	"fmt"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetJob Gets job using lister if present
func (kube *Kube) GetJob(namespace, name string) (*v1.RadixJob, error) {
	var job *v1.RadixJob
	var err error

	if kube.RjLister != nil {
		job, err = kube.RjLister.RadixJobs(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		job, err = kube.radixclient.RadixV1().RadixJobs(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return job, nil
}

// ListRadixJobs Gets jobs using lister if present
func (kube *Kube) ListRadixJobs(namespace string) ([]*v1.RadixJob, error) {
	var allRJs []*v1.RadixJob
	var err error

	if kube.RjLister != nil {
		allRJs, err = kube.RjLister.RadixJobs(namespace).List(labels.NewSelector())
		if err != nil {
			err = fmt.Errorf("Failed to get all RadixJobs. Error was %v", err)
		}
	} else {
		rjs, err := kube.radixclient.RadixV1().RadixJobs(namespace).List(metav1.ListOptions{})
		if err != nil {
			err = fmt.Errorf("Failed to get all RadixJobs. Error was %v", err)
		}

		allRJs = slice.PointersOf(rjs.Items).([]*v1.RadixJob)
	}

	return allRJs, err
}
