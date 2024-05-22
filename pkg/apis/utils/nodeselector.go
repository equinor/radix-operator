package utils

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	corev1 "k8s.io/api/core/v1"
)

func GetNodeSelector() map[string]string {
	return map[string]string{corev1.LabelArchStable: defaults.DefaultNodeSelectorArchitecture}
}
