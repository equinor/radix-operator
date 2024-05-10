package deployment

import (
	"context"
	"strconv"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestScaler_DefaultConfigurationDoesNotHaveMemoryScaling(t *testing.T) {
	tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := setupTest(t)
	rrBuilder := utils.ARadixRegistration().WithName("someapp")
	raBuilder := utils.ARadixApplication().
		WithRadixRegistration(rrBuilder)

	var testScenarios = []struct {
		name                 string
		cpuTarget            *int32
		expectedCpuTarget    *int
		memoryTarget         *int32
		expectedMemoryTarget *int
	}{
		{"cpu and memory are nil, cpu defaults to 80", nil, numbers.IntPtr(80), nil, nil},
		{"cpu is nil and memory is non-nil", nil, nil, numbers.Int32Ptr(70), numbers.IntPtr(70)},
		{"cpu is non-nil and memory is nil", numbers.Int32Ptr(68), numbers.IntPtr(68), nil, nil},
		{"cpu and memory are non-nil", numbers.Int32Ptr(68), numbers.IntPtr(68), numbers.Int32Ptr(70), numbers.IntPtr(70)},
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
			_, err = kubeclient.AutoscalingV2().HorizontalPodAutoscalers(rd.GetNamespace()).Get(context.Background(), rd.Spec.Components[0].GetName(), metav1.GetOptions{})
			assert.True(t, errors.IsNotFound(err))

			scaler, err := kedaClient.KedaV1alpha1().ScaledObjects(rd.GetNamespace()).Get(context.Background(), rd.Spec.Components[0].GetName(), metav1.GetOptions{})
			require.NoError(t, err)

			var actualCpuTarget, actualMemoryTarget *int
			if memoryMetric := utils.GetScaleTrigger(scaler, "memory", "memory"); memoryMetric != nil {
				value, err := strconv.Atoi(memoryMetric.Metadata["value"])
				require.NoError(t, err)
				actualMemoryTarget = &value
			}
			if cpuMetric := utils.GetScaleTrigger(scaler, "cpu", "cpu"); cpuMetric != nil {
				value, err := strconv.Atoi(cpuMetric.Metadata["value"])
				require.NoError(t, err)
				actualCpuTarget = &value
			}

			if testcase.expectedCpuTarget == nil {
				assert.Nil(t, actualCpuTarget)
			} else {
				require.NotNil(t, actualCpuTarget)
				assert.Equal(t, *testcase.expectedCpuTarget, *actualCpuTarget)
			}

			if testcase.expectedMemoryTarget == nil {
				assert.Nil(t, actualMemoryTarget)
			} else {
				require.NotNil(t, actualMemoryTarget)
				assert.Equal(t, *testcase.expectedMemoryTarget, *actualMemoryTarget)
			}
		})
	}
}
