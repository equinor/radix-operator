package utils

import (
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"k8s.io/api/policy/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// GetPDBName returns a PodDisruptionBudget name
func GetPDBName(componentName string) string {
	return fmt.Sprintf("%s-pdb", componentName)
}

// GetPDBConfig returns a standard PodDisruptionBudget configuration
func GetPDBConfig(componentName string, namespace string) *v1.PodDisruptionBudget {
	pdb := &v1.PodDisruptionBudget{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PodDisruptionBudget",
			APIVersion: "policy/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPDBName(componentName),
			Namespace: namespace,
			Labels:    map[string]string{kube.RadixComponentLabel: componentName},
		},
		Spec: v1.PodDisruptionBudgetSpec{
			MinAvailable: &intstr.IntOrString{
				IntVal: 1,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{kube.RadixComponentLabel: componentName},
			},
		},
	}
	return pdb
}
