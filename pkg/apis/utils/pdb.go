package utils

import (
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"k8s.io/api/policy/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// GetPDBName returns a PodDisruptionBudget name
func GetPDBName(componentName string) string {
	return fmt.Sprintf("%s-pdb", componentName)
}

// GetPDBConfig returns a standard PodDisruptionBudget configuration
func GetPDBConfig(componentName string, namespace string) *v1.PodDisruptionBudget {
	pdb := &v1.PodDisruptionBudget{
		TypeMeta: v12.TypeMeta{
			Kind:       "PodDisruptionBudget",
			APIVersion: "policy/v1",
		},
		ObjectMeta: v12.ObjectMeta{
			Name:      GetPDBName(componentName),
			Namespace: namespace,
			Labels:    map[string]string{kube.RadixComponentLabel: componentName},
		},
		Spec: v1.PodDisruptionBudgetSpec{
			MinAvailable: &intstr.IntOrString{
				IntVal: 1,
			},
			Selector: &v12.LabelSelector{
				MatchLabels: map[string]string{kube.RadixComponentLabel: componentName},
			},
		},
	}
	return pdb
}
