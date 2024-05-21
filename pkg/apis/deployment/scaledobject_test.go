package deployment_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	"github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestScaler_DefaultConfigurationDoesNotHaveMemoryScaling(t *testing.T) {
	tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := deployment.SetupTest(t)
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
		{"cpu and memory are nil, cpu defaults to 80", nil, numbers.IntPtr(radixv1.DefaultTargetCPUUtilizationPercentage), nil, nil},
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

			rd, err := deployment.ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, rdBuilder)
			assert.NoError(t, err)
			_, err = kubeclient.AutoscalingV2().HorizontalPodAutoscalers(rd.GetNamespace()).Get(context.Background(), rd.Spec.Components[0].GetName(), metav1.GetOptions{})
			assert.True(t, errors.IsNotFound(err))

			scaler, err := kedaClient.KedaV1alpha1().ScaledObjects(rd.GetNamespace()).Get(context.Background(), rd.Spec.Components[0].GetName(), metav1.GetOptions{})
			require.NoError(t, err)

			var actualCpuTarget, actualMemoryTarget *int
			if memoryMetric := getFirstTriggerByType(scaler, "memory"); memoryMetric != nil {
				value, err := strconv.Atoi(memoryMetric.Metadata["value"])
				require.NoError(t, err)
				actualMemoryTarget = &value
			}
			if cpuMetric := getFirstTriggerByType(scaler, "cpu"); cpuMetric != nil {
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

func TestHorizontalAutoscalingConfig(t *testing.T) {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := deployment.SetupTest(t)
	defer deployment.TeardownTest()
	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentOneName := "componentOneName"
	componentTwoName := "componentTwoName"
	componentThreeName := "componentThreeName"
	minReplicas := int32(2)
	maxReplicas := int32(4)

	// Test
	_, err := deployment.ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithReplicas(test.IntPtr(0)).
				WithHorizontalScaling(&minReplicas, maxReplicas, nil, nil),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("http").
				WithReplicas(test.IntPtr(1)).
				WithHorizontalScaling(&minReplicas, maxReplicas, nil, nil),
			utils.NewDeployComponentBuilder().
				WithName(componentThreeName).
				WithPort("http", 6379).
				WithPublicPort("http").
				WithReplicas(test.IntPtr(1)).
				WithHorizontalScaling(&minReplicas, maxReplicas, nil, nil)))
	require.NoError(t, err)

	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)
	t.Run("validate scaled objects", func(t *testing.T) {
		scalers, _ := kedaClient.KedaV1alpha1().ScaledObjects(envNamespace).List(context.Background(), metav1.ListOptions{})
		require.Equal(t, 2, len(scalers.Items), "Number of horizontal pod autoscalers wasn't as expected")
		assert.False(t, scaledObjectByNameExists(componentOneName, scalers), "componentOneName horizontal pod autoscaler should not exist")
		assert.True(t, scaledObjectByNameExists(componentTwoName, scalers), "componentTwoName horizontal pod autoscaler should exist")
		assert.True(t, scaledObjectByNameExists(componentThreeName, scalers), "componentThreeName horizontal pod autoscaler should exist")
		assert.Equal(t, int32(2), *getScaledObjectByName(componentTwoName, scalers).Spec.MinReplicaCount, "componentTwoName horizontal pod autoscaler config is incorrect")
	})

	// Test - remove HPA from component three
	_, err = deployment.ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithReplicas(test.IntPtr(0)).
				WithHorizontalScaling(&minReplicas, maxReplicas, nil, nil),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("http").
				WithReplicas(test.IntPtr(1)).
				WithHorizontalScaling(&minReplicas, maxReplicas, nil, nil),
			utils.NewDeployComponentBuilder().
				WithName(componentThreeName).
				WithPort("http", 6379).
				WithPublicPort("http").
				WithReplicas(test.IntPtr(1))))
	require.NoError(t, err)

	t.Run("validate scaled objects after reconfiguration", func(t *testing.T) {
		scalers, _ := kedaClient.KedaV1alpha1().ScaledObjects(envNamespace).List(context.Background(), metav1.ListOptions{})
		require.Equal(t, 1, len(scalers.Items), "Number of horizontal pod autoscalers wasn't as expected")
		assert.False(t, scaledObjectByNameExists(componentOneName, scalers), "componentOneName horizontal pod autoscaler should not exist")
		assert.True(t, scaledObjectByNameExists(componentTwoName, scalers), "componentTwoName horizontal pod autoscaler should exist")
		assert.False(t, scaledObjectByNameExists(componentThreeName, scalers), "componentThreeName horizontal pod autoscaler should not exist")
		assert.Equal(t, int32(2), *getScaledObjectByName(componentTwoName, scalers).Spec.MinReplicaCount, "componentTwoName horizontal pod autoscaler config is incorrect")
	})

}

func TestScalerTriggers(t *testing.T) {
	tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := deployment.SetupTest(t)
	componentName := "a-component-name"
	rrBuilder := utils.ARadixRegistration().WithName("someapp")
	raBuilder := utils.ARadixApplication().
		WithRadixRegistration(rrBuilder)

	var testScenarios = []struct {
		name        string
		builder     *utils.HorizontalScalingBuilderStruct
		expected    v1alpha1.ScaleTriggers
		expecedAuth *v1alpha1.TriggerAuthenticationSpec
	}{
		{
			name:    "CPU",
			builder: utils.NewHorizontalScalingBuilder().WithCPUTrigger(80),
			expected: v1alpha1.ScaleTriggers{Name: "cpu", Type: "cpu", MetricType: v2.UtilizationMetricType, Metadata: map[string]string{
				"value": "80",
			}},
		},
		{
			name:    "Memory",
			builder: utils.NewHorizontalScalingBuilder().WithMemoryTrigger(80),
			expected: v1alpha1.ScaleTriggers{Name: "memory", Type: "memory", MetricType: v2.UtilizationMetricType, Metadata: map[string]string{
				"value": "80",
			}},
		},
		{
			name:    "Cron",
			builder: utils.NewHorizontalScalingBuilder().WithCRONTrigger("10 * * * *", "20 * * * *", "Europe/Oslo", 30),
			expected: v1alpha1.ScaleTriggers{Name: "cron", Type: "cron", Metadata: map[string]string{
				"start":           "10 * * * *",
				"stop":            "20 * * * *",
				"timezone":        "Europe/Oslo",
				"desiredReplicas": "30",
			}},
		},
		{
			name:    "AzureServiceBus-Queue",
			builder: utils.NewHorizontalScalingBuilder().WithAzureServiceBusTrigger("anamespace", "abcd", pointers.Ptr("queue-name"), nil, nil, pointers.Ptr(5), pointers.Ptr(10)),
			expected: v1alpha1.ScaleTriggers{
				Name: "azure-service-bus",
				Type: "azure-servicebus",
				Metadata: map[string]string{
					"queueName":              "queue-name",
					"namespace":              "anamespace",
					"messageCount":           "5",
					"activationMessageCount": "10",
				},
				AuthenticationRef: &v1alpha1.AuthenticationRef{Name: utils.GetTriggerAuthenticationName(componentName, "azure-service-bus"), Kind: "TriggerAuthentication"},
			},
			expecedAuth: &v1alpha1.TriggerAuthenticationSpec{
				PodIdentity: &v1alpha1.AuthPodIdentity{
					Provider:   "azure-workload",
					IdentityID: pointers.Ptr("abcd"),
				},
			},
		},
		{
			name:    "AzureServiceBus-Topic",
			builder: utils.NewHorizontalScalingBuilder().WithAzureServiceBusTrigger("anamespace", "abcd", nil, pointers.Ptr("topic-name"), pointers.Ptr("subscription-name"), pointers.Ptr(5), pointers.Ptr(10)),
			expected: v1alpha1.ScaleTriggers{
				Name: "azure-service-bus",
				Type: "azure-servicebus",
				Metadata: map[string]string{
					"topicName":              "topic-name",
					"subscriptionName":       "subscription-name",
					"namespace":              "anamespace",
					"messageCount":           "5",
					"activationMessageCount": "10",
				},
				AuthenticationRef: &v1alpha1.AuthenticationRef{Name: utils.GetTriggerAuthenticationName(componentName, "azure-service-bus"), Kind: "TriggerAuthentication"},
			},
			expecedAuth: &v1alpha1.TriggerAuthenticationSpec{
				PodIdentity: &v1alpha1.AuthPodIdentity{
					Provider:   "azure-workload",
					IdentityID: pointers.Ptr("abcd"),
				},
			},
		},
	}

	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			// Test that memory scaling is not enabled by default
			rdBuilder := utils.ARadixDeployment().
				WithRadixApplication(raBuilder).
				WithComponents(utils.NewDeployComponentBuilder().
					WithName(componentName).
					WithHorizontalScalingBuilder(
						testcase.builder.
							WithMinReplicas(1).
							WithMaxReplicas(3).
							WithPollingInterval(4).
							WithCooldownPeriod(5)))

			rd, err := deployment.ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, rdBuilder)
			assert.NoError(t, err)

			scaler, err := kedaClient.KedaV1alpha1().ScaledObjects(rd.GetNamespace()).Get(context.Background(), rd.Spec.Components[0].GetName(), metav1.GetOptions{})
			require.NoError(t, err)
			require.Len(t, scaler.Spec.Triggers, 1)
			trigger := scaler.Spec.Triggers[0]

			assert.Equal(t, testcase.expected, trigger)

			if testcase.expecedAuth == nil {
				assert.Nil(t, testcase.expecedAuth)
			} else {
				actualAuth, err := kedaClient.KedaV1alpha1().TriggerAuthentications(rd.GetNamespace()).Get(context.Background(), testcase.expected.AuthenticationRef.Name, metav1.GetOptions{})
				require.NoError(t, err)

				assert.Equal(t, *testcase.expecedAuth, actualAuth.Spec)
			}

		})
	}
}

// getFirstTriggerByType returns the trigger spec for the given name  and type or nil if not found
func getFirstTriggerByType(scaledObject *v1alpha1.ScaledObject, triggerType string) *v1alpha1.ScaleTriggers {
	for _, trigger := range scaledObject.Spec.Triggers {
		if trigger.Type == triggerType {
			return &trigger
		}
	}
	return nil
}

func scaledObjectByNameExists(name string, scalers *v1alpha1.ScaledObjectList) bool {
	return getScaledObjectByName(name, scalers) != nil
}

func getScaledObjectByName(name string, scalers *v1alpha1.ScaledObjectList) *v1alpha1.ScaledObject {
	for _, scaler := range scalers.Items {
		if scaler.Name == name {
			return &scaler
		}
	}

	return nil
}
