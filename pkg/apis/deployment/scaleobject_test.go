package deployment

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHpa_DefaultConfigurationDoesNotHaveMemoryScaling(t *testing.T) {
	tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := setupTest(t)
	rrBuilder := utils.ARadixRegistration().WithName("someapp")
	raBuilder := utils.ARadixApplication().
		WithRadixRegistration(rrBuilder)

	var testScenarios = []struct {
		name                 string
		cpuTarget            *int32
		expectedCpuTarget    *int32
		memoryTarget         *int32
		expectedMemoryTarget *int32
	}{
		{"cpu and memory are nil, cpu defaults to 80", nil, numbers.Int32Ptr(80), nil, nil},
		{"cpu is nil and memory is non-nil", nil, nil, numbers.Int32Ptr(70), numbers.Int32Ptr(70)},
		{"cpu is non-nil and memory is nil", numbers.Int32Ptr(68), numbers.Int32Ptr(68), nil, nil},
		{"cpu and memory are non-nil", numbers.Int32Ptr(68), numbers.Int32Ptr(68), numbers.Int32Ptr(70), numbers.Int32Ptr(70)},
	}
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			// Test that memory scaling is not enabled by default
			rdBuilder := utils.ARadixDeployment().
				WithRadixApplication(raBuilder).
				WithComponents(utils.NewDeployComponentBuilder().
					WithHorizontalScaling(numbers.Int32Ptr(1), 3, testcase.cpuTarget, testcase.memoryTarget))

			rd, err := applyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, rdBuilder)
			assert.NoError(t, err)
			hpa, err := kubeclient.AutoscalingV2().HorizontalPodAutoscalers(rd.GetNamespace()).Get(context.Background(), rd.Spec.Components[0].GetName(), metav1.GetOptions{})
			assert.NoError(t, err)

			var actualCpuTarget, actualMemoryTarget *int32
			if memoryMetric := utils.GetHpaMetric(hpa, corev1.ResourceMemory); memoryMetric != nil {
				actualMemoryTarget = memoryMetric.Resource.Target.AverageUtilization
			}
			if cpuMetric := utils.GetHpaMetric(hpa, corev1.ResourceCPU); cpuMetric != nil {
				actualCpuTarget = cpuMetric.Resource.Target.AverageUtilization
			}

			assert.Equal(t, testcase.expectedCpuTarget, actualCpuTarget)
			assert.Equal(t, testcase.expectedMemoryTarget, actualMemoryTarget)
		})
	}
}
