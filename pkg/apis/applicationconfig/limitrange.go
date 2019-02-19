package applicationconfig

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	limitRangeName       = "mem-cpu-limit-range"
	defaultCPU           = "0.5"
	defaultMemory        = "300M"
	defaultCPURequest    = "0.25"
	defaultMemoryRequest = "256M"
)

func (app *ApplicationConfig) createLimitRangeOnNamespace(namespace string) error {
	defaultResources := make(corev1.ResourceList)
	defaultResources[corev1.ResourceCPU] = resource.MustParse(defaultCPU)
	defaultResources[corev1.ResourceMemory] = resource.MustParse(defaultMemory)

	defaultRequest := make(corev1.ResourceList)
	defaultRequest[corev1.ResourceCPU] = resource.MustParse(defaultCPURequest)
	defaultRequest[corev1.ResourceMemory] = resource.MustParse(defaultMemoryRequest)

	limitRange := &corev1.LimitRange{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "LimitRange",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      limitRangeName,
			Namespace: namespace,
			Labels: map[string]string{
				kube.RadixAppLabel: app.config.Name,
			},
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				corev1.LimitRangeItem{
					Type:           corev1.LimitTypeContainer,
					Default:        defaultResources,
					DefaultRequest: defaultRequest,
				},
			},
		},
	}

	return app.kubeutil.ApplyLimitRange(namespace, limitRange)
}
