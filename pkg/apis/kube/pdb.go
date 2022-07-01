package kube

import (
	"context"
	"github.com/equinor/radix-common/utils/slice"
	v12 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (kubeutil *Kube) ListPodDisruptionBudgets(namespace string) ([]*v12.PodDisruptionBudget, error) {
	list, err := kubeutil.kubeClient.PolicyV1().PodDisruptionBudgets(namespace).List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		return nil, err
	}
	pdbs := slice.PointersOf(list.Items).([]*v12.PodDisruptionBudget)
	return pdbs, nil
}
