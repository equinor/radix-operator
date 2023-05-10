package deployment

import (
	"context"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestHpa_DefaultConfigurationDoesNotHaveMemoryScaling(t *testing.T) {
	tu, kubeclient, kubeUtil, radixclient, prometheusclient, _ := setupTest()
	rrBuilder := utils.ARadixRegistration().WithName("someapp")
	raBuilder := utils.ARadixApplication().
		WithRadixRegistration(rrBuilder)

	// Test that memory scaling is not enabled by default
	cpuTarget := 68
	rdBuilder := utils.ARadixDeployment().
		WithRadixApplication(raBuilder).
		WithComponents(utils.NewDeployComponentBuilder().WithHorizontalScaling(numbers.Int32Ptr(1), 3, numbers.Int32Ptr(int32(cpuTarget)), nil))

	rd, err := applyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, prometheusclient, rdBuilder)
	require.NoError(t, err)
	assert.Nil(t, rd.Spec.Components[0].HorizontalScaling.RadixHorizontalScalingResources.Memory)

	hpa, err := kubeclient.AutoscalingV2().HorizontalPodAutoscalers(rd.GetNamespace()).Get(context.TODO(), rd.Spec.Components[0].GetName(), metav1.GetOptions{})
	assert.NoError(t, err)
	memoryMetric := getHpaMetric(hpa, corev1.ResourceMemory)
	assert.Nil(t, memoryMetric)

	cpuMetric := getHpaMetric(hpa, corev1.ResourceCPU)
	assert.NotNil(t, cpuMetric)
	assert.Equal(t, int32(cpuTarget), *cpuMetric.Resource.Target.AverageUtilization)

	// Test that memory scaling is enabled when configured
	cpuTarget = 70
	memoryTarget := 75
	rdBuilder = utils.ARadixDeployment().
		WithRadixApplication(raBuilder).
		WithComponents(utils.NewDeployComponentBuilder().WithHorizontalScaling(numbers.Int32Ptr(1), 3, numbers.Int32Ptr(int32(cpuTarget)), numbers.Int32Ptr(int32(memoryTarget))))

	rd, err = applyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, prometheusclient, rdBuilder)
	require.NoError(t, err)

	hpa, err = kubeclient.AutoscalingV2().HorizontalPodAutoscalers(rd.GetNamespace()).Get(context.TODO(), rd.Spec.Components[0].GetName(), metav1.GetOptions{})
	assert.NoError(t, err)
	memoryMetric = getHpaMetric(hpa, corev1.ResourceMemory)
	assert.NotNil(t, memoryMetric)
	assert.Equal(t, int32(memoryTarget), *memoryMetric.Resource.Target.AverageUtilization)

	cpuMetric = getHpaMetric(hpa, corev1.ResourceCPU)
	assert.NotNil(t, cpuMetric)
	assert.Equal(t, int32(cpuTarget), *cpuMetric.Resource.Target.AverageUtilization)

	// Test that cpu defaults to 80 if not specified, and that memory disappears if not specified
	rdBuilder = utils.ARadixDeployment().
		WithRadixApplication(raBuilder).
		WithComponents(utils.NewDeployComponentBuilder().WithHorizontalScaling(numbers.Int32Ptr(1), 3, nil, nil))

	rd, err = applyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, prometheusclient, rdBuilder)
	require.NoError(t, err)

	hpa, err = kubeclient.AutoscalingV2().HorizontalPodAutoscalers(rd.GetNamespace()).Get(context.TODO(), rd.Spec.Components[0].GetName(), metav1.GetOptions{})
	assert.NoError(t, err)
	memoryMetric = getHpaMetric(hpa, corev1.ResourceMemory)
	assert.Nil(t, memoryMetric)

	cpuMetric = getHpaMetric(hpa, corev1.ResourceCPU)
	assert.NotNil(t, cpuMetric)
	assert.Equal(t, targetCPUUtilizationPercentage, *cpuMetric.Resource.Target.AverageUtilization)
}

func getHpaMetric(hpa *autoscalingv2.HorizontalPodAutoscaler, resourceName corev1.ResourceName) *autoscalingv2.MetricSpec {
	for _, metric := range hpa.Spec.Metrics {
		if metric.Resource != nil && metric.Resource.Name == resourceName {
			return &metric
		}
	}
	return nil
}
