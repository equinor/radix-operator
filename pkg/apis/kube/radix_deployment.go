package kube

import (
	"fmt"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetRadixDeployment Gets deployment using lister if present
func (kube *Kube) GetRadixDeployment(namespace, name string) (*v1.RadixDeployment, error) {
	var deployment *v1.RadixDeployment
	var err error

	if kube.RdLister != nil {
		deployment, err = kube.RdLister.RadixDeployments(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		deployment, err = kube.radixclient.RadixV1().RadixDeployments(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return deployment, nil
}

// ListRadixDeployments Gets deployments using lister if present
func (kube *Kube) ListRadixDeployments(namespace string) ([]*v1.RadixDeployment, error) {
	var allRDs []*v1.RadixDeployment
	var err error

	if kube.RdLister != nil {
		allRDs, err = kube.RdLister.RadixDeployments(namespace).List(labels.NewSelector())
		if err != nil {
			err = fmt.Errorf("Failed to get all RadixDeployments. Error was %v", err)
		}
	} else {
		rds, err := kube.radixclient.RadixV1().RadixDeployments(namespace).List(metav1.ListOptions{})
		if err != nil {
			err = fmt.Errorf("Failed to get all RadixDeployments. Error was %v", err)
		}

		allRDs = slice.PointersOf(rds.Items).([]*v1.RadixDeployment)
	}

	return allRDs, err
}
