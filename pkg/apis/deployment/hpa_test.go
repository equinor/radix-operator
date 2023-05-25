package deployment

import (
	"context"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestHpa_DefaultConfigurationDoesNotHaveMemoryScaling(t *testing.T) {
	tu, kubeclient, kubeUtil, radixclient, prometheusclient, _ := setupTest()
	rrBuilder := utils.ARadixRegistration().WithName("someapp")
	raBuilder := utils.ARadixApplication().
		WithRadixRegistration(rrBuilder)

	var testScenarios = []struct {
		name                         string
		cpuTarget                    *int32
		expectedCpuTarget            int32
		memoryTarget                 *int32
		cpuTargetShouldBeDefined     bool
		cpuTargetShouldBeDefinedInRd bool
		memoryTargetShouldBeDefined  bool
	}{
		{"cpuTarget is present when configured, and memoryTarget is not", numbers.Int32Ptr(68), 68, nil, true, true, false},
		// Test that memory scaling is enabled when configured
		{"memory defined when configured", numbers.Int32Ptr(68), 68, numbers.Int32Ptr(70), true, true, true},
		// Test that cpu defaults to 80 if not specified, and that memory disappears if not specified
		{"cpu defaults to 80 if not specified", nil, 80, nil, true, false, false},
		// Test that cpu disappears if memory is specified while cpu is not
		{"cpu disappears if memory is specified while cpu is not", nil, -3, numbers.Int32Ptr(70), false, false, true},
	}
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			// Test that memory scaling is not enabled by default
			rdBuilder := utils.ARadixDeployment().
				WithRadixApplication(raBuilder).
				WithComponents(utils.NewDeployComponentBuilder().
					WithHorizontalScaling(numbers.Int32Ptr(1), 3, testcase.cpuTarget, testcase.memoryTarget))

			rd, err := applyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, prometheusclient, rdBuilder)
			assert.NoError(t, err)
			hpa, err := kubeclient.AutoscalingV2().HorizontalPodAutoscalers(rd.GetNamespace()).Get(context.TODO(), rd.Spec.Components[0].GetName(), metav1.GetOptions{})
			assert.NoError(t, err)
			memoryMetric := utils.GetHpaMetric(hpa, corev1.ResourceMemory)
			cpuMetric := utils.GetHpaMetric(hpa, corev1.ResourceCPU)
			assert.Equal(t, testcase.memoryTargetShouldBeDefined, memoryMetric != nil)
			assert.Equal(t, testcase.cpuTargetShouldBeDefined, cpuMetric != nil)
			if testcase.memoryTargetShouldBeDefined {
				assert.Equal(t, testcase.memoryTarget, rd.Spec.Components[0].HorizontalScaling.RadixHorizontalScalingResources.Memory.AverageUtilization)
				assert.Equal(t, testcase.memoryTarget, memoryMetric.Resource.Target.AverageUtilization)
			} else {
				assert.Nil(t, rd.Spec.Components[0].HorizontalScaling.RadixHorizontalScalingResources.Memory)
			}
			if testcase.cpuTargetShouldBeDefinedInRd {
				assert.Equal(t, testcase.expectedCpuTarget, *rd.Spec.Components[0].HorizontalScaling.RadixHorizontalScalingResources.Cpu.AverageUtilization)
			}
			if testcase.cpuTargetShouldBeDefined {
				assert.Equal(t, testcase.expectedCpuTarget, *cpuMetric.Resource.Target.AverageUtilization)
			} else {
				assert.Nil(t, rd.Spec.Components[0].HorizontalScaling.RadixHorizontalScalingResources.Cpu)
			}
		})
	}

}
