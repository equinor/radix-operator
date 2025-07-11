package deployment_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	"github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
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
		cpuTarget            *int
		expectedCpuTarget    *int
		memoryTarget         *int
		expectedMemoryTarget *int
	}{
		{"cpu and memory are nil, cpu defaults to 80", nil, numbers.IntPtr(radixv1.DefaultTargetCPUUtilizationPercentage), nil, nil},
		{"cpu is nil and memory is non-nil", nil, nil, numbers.IntPtr(70), numbers.IntPtr(70)},
		{"cpu is non-nil and memory is nil", numbers.IntPtr(68), numbers.IntPtr(68), nil, nil},
		{"cpu and memory are non-nil", numbers.IntPtr(68), numbers.IntPtr(68), numbers.IntPtr(70), numbers.IntPtr(70)},
	}

	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			// Test that memory scaling is not enabled by default
			scalerBUilder := utils.NewHorizontalScalingBuilder().WithMinReplicas(1).WithMaxReplicas(3)
			if testcase.cpuTarget != nil {
				scalerBUilder.WithCPUTrigger(*testcase.cpuTarget)
			}
			if testcase.memoryTarget != nil {
				scalerBUilder.WithMemoryTrigger(*testcase.memoryTarget)
			}

			rdBuilder := utils.ARadixDeployment().
				WithRadixApplication(raBuilder).
				WithComponents(utils.NewDeployComponentBuilder().WithHorizontalScaling(scalerBUilder.Build()))

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

	// Test
	_, err := deployment.ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithReplicas(pointers.Ptr(1)).
				WithReplicasOverride(pointers.Ptr(0)).
				WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(2).WithMaxReplicas(4).Build()),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("http").
				WithReplicas(pointers.Ptr(1)).
				WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(2).WithMaxReplicas(4).Build()),
			utils.NewDeployComponentBuilder().
				WithName(componentThreeName).
				WithPort("http", 6379).
				WithPublicPort("http").
				WithReplicas(pointers.Ptr(1)).
				WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(2).WithMaxReplicas(4).WithAzureServiceBusTrigger("test", "abcd", "queue", "", "", "", nil, nil).Build())))
	require.NoError(t, err)

	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)
	scalers, _ := kedaClient.KedaV1alpha1().ScaledObjects(envNamespace).List(context.Background(), metav1.ListOptions{})
	authTriggers, _ := kedaClient.KedaV1alpha1().TriggerAuthentications(envNamespace).List(context.Background(), metav1.ListOptions{})
	require.Equal(t, 2, len(scalers.Items), "Number of horizontal pod autoscalers wasn't as expected")
	assert.False(t, scaledObjectByNameExists(componentOneName, scalers), "componentOneName horizontal pod autoscaler should not exist")
	assert.True(t, scaledObjectByNameExists(componentTwoName, scalers), "componentTwoName horizontal pod autoscaler should exist")
	assert.True(t, scaledObjectByNameExists(componentThreeName, scalers), "componentThreeName horizontal pod autoscaler should exist")
	assert.Equal(t, int32(2), *getScaledObjectByName(componentTwoName, scalers).Spec.MinReplicaCount, "componentTwoName horizontal pod autoscaler config is incorrect")
	require.Len(t, authTriggers.Items, 1, "componentThree requires AuthTrigger for Azure Service Bus")

	// Test - remove HPA from component three
	_, err = deployment.ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithReplicas(pointers.Ptr(1)).
				WithReplicasOverride(pointers.Ptr(0)).
				WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(2).WithMaxReplicas(4).Build()),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("http").
				WithReplicas(pointers.Ptr(1)).
				WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(2).WithMaxReplicas(4).Build()),
			utils.NewDeployComponentBuilder().
				WithName(componentThreeName).
				WithPort("http", 6379).
				WithPublicPort("http").
				WithReplicas(pointers.Ptr(1))))
	require.NoError(t, err)

	scalers, _ = kedaClient.KedaV1alpha1().ScaledObjects(envNamespace).List(context.Background(), metav1.ListOptions{})
	authTriggers, _ = kedaClient.KedaV1alpha1().TriggerAuthentications(envNamespace).List(context.Background(), metav1.ListOptions{})
	require.Equal(t, 1, len(scalers.Items), "Number of horizontal pod autoscalers wasn't as expected")
	assert.False(t, scaledObjectByNameExists(componentOneName, scalers), "componentOneName horizontal pod autoscaler should not exist")
	assert.True(t, scaledObjectByNameExists(componentTwoName, scalers), "componentTwoName horizontal pod autoscaler should exist")
	assert.False(t, scaledObjectByNameExists(componentThreeName, scalers), "componentThreeName horizontal pod autoscaler should not exist")
	assert.Equal(t, int32(2), *getScaledObjectByName(componentTwoName, scalers).Spec.MinReplicaCount, "componentTwoName horizontal pod autoscaler config is incorrect")
	assert.Len(t, authTriggers.Items, 0, "AuthTrigger should be removed when not required anymore by componentThree")

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
			expected: v1alpha1.ScaleTriggers{Name: "cpu", Type: "cpu", MetricType: autoscalingv2.UtilizationMetricType, Metadata: map[string]string{
				"value": "80",
			}},
		},
		{
			name:    "Memory",
			builder: utils.NewHorizontalScalingBuilder().WithMemoryTrigger(80),
			expected: v1alpha1.ScaleTriggers{Name: "memory", Type: "memory", MetricType: autoscalingv2.UtilizationMetricType, Metadata: map[string]string{
				"value": "80",
			}},
		},
		{
			name:    "Cron",
			builder: utils.NewHorizontalScalingBuilder().WithCRONTrigger("10 * * * *", "20 * * * *", "Europe/Oslo", 30),
			expected: v1alpha1.ScaleTriggers{Name: "cron", Type: "cron", Metadata: map[string]string{
				"start":           "10 * * * *",
				"end":             "20 * * * *",
				"timezone":        "Europe/Oslo",
				"desiredReplicas": "30",
			}},
		},
		{
			name:    "AzureServiceBus-Queue Workload Identity",
			builder: utils.NewHorizontalScalingBuilder().WithAzureServiceBusTrigger("anamespace", "abcd", "queue-name", "", "", "", pointers.Ptr(5), pointers.Ptr(10)),
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
			name:    "AzureServiceBus-Queue Connection String",
			builder: utils.NewHorizontalScalingBuilder().WithAzureServiceBusTrigger("anamespace", "", "queue-name", "", "", "CONNECTION_STRING", pointers.Ptr(5), pointers.Ptr(10)),
			expected: v1alpha1.ScaleTriggers{
				Name: "azure-service-bus",
				Type: "azure-servicebus",
				Metadata: map[string]string{
					"queueName":              "queue-name",
					"namespace":              "anamespace",
					"messageCount":           "5",
					"activationMessageCount": "10",
					"connectionFromEnv":      "CONNECTION_STRING",
				},
				AuthenticationRef: nil,
			},
			expecedAuth: nil,
		},
		{
			name:    "AzureServiceBus-Topic",
			builder: utils.NewHorizontalScalingBuilder().WithAzureServiceBusTrigger("anamespace", "abcd", "", "topic-name", "subscription-name", "", pointers.Ptr(5), pointers.Ptr(10)),
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
		{
			name: "AzureEventHub with Workload identity",
			builder: utils.NewHorizontalScalingBuilder().WithAzureEventHubTrigger("some-client-id", &radixv1.RadixHorizontalScalingAzureEventHubTrigger{
				EventHubNamespace:                   "some-eventhub-namespace",
				EventHubName:                        "some-eventhub-name",
				ConsumerGroup:                       "some-consumer-group",
				StorageAccount:                      "some-storage-account",
				Container:                           "some-container",
				CheckpointStrategy:                  "goSdk",
				UnprocessedEventThreshold:           pointers.Ptr(100),
				ActivationUnprocessedEventThreshold: pointers.Ptr(2),
			}),
			expected: v1alpha1.ScaleTriggers{
				Name: "azure-event-hub",
				Type: "azure-eventhub",
				Metadata: map[string]string{
					"eventHubNamespace":                   "some-eventhub-namespace",
					"eventHubName":                        "some-eventhub-name",
					"consumerGroup":                       "some-consumer-group",
					"storageAccountName":                  "some-storage-account",
					"checkpointStrategy":                  "goSdk",
					"blobContainer":                       "some-container",
					"unprocessedEventThreshold":           "100",
					"activationUnprocessedEventThreshold": "2",
				},
				AuthenticationRef: &v1alpha1.AuthenticationRef{Name: utils.GetTriggerAuthenticationName(componentName, "azure-event-hub"), Kind: "TriggerAuthentication"},
			},
			expecedAuth: &v1alpha1.TriggerAuthenticationSpec{
				PodIdentity: &v1alpha1.AuthPodIdentity{
					Provider:   "azure-workload",
					IdentityID: pointers.Ptr("some-client-id"),
				},
			},
		},
		{
			name: "AzureEventHub with Workload identity and event hub namespace and name from env-vars",
			builder: utils.NewHorizontalScalingBuilder().WithAzureEventHubTrigger("some-client-id", &radixv1.RadixHorizontalScalingAzureEventHubTrigger{
				EventHubNamespaceFromEnv:            "EVENTHUB_NAMESPACE",
				EventHubNameFromEnv:                 "EVENTHUB_NAME",
				ConsumerGroup:                       "some-consumer-group",
				StorageAccount:                      "some-storage-account",
				Container:                           "some-container",
				CheckpointStrategy:                  "goSdk",
				UnprocessedEventThreshold:           pointers.Ptr(100),
				ActivationUnprocessedEventThreshold: pointers.Ptr(2),
			}),
			expected: v1alpha1.ScaleTriggers{
				Name: "azure-event-hub",
				Type: "azure-eventhub",
				Metadata: map[string]string{
					"eventHubNamespaceFromEnv":            "EVENTHUB_NAMESPACE",
					"eventHubNameFromEnv":                 "EVENTHUB_NAME",
					"consumerGroup":                       "some-consumer-group",
					"storageAccountName":                  "some-storage-account",
					"checkpointStrategy":                  "goSdk",
					"blobContainer":                       "some-container",
					"unprocessedEventThreshold":           "100",
					"activationUnprocessedEventThreshold": "2",
				},
				AuthenticationRef: &v1alpha1.AuthenticationRef{Name: utils.GetTriggerAuthenticationName(componentName, "azure-event-hub"), Kind: "TriggerAuthentication"},
			},
			expecedAuth: &v1alpha1.TriggerAuthenticationSpec{
				PodIdentity: &v1alpha1.AuthPodIdentity{
					Provider:   "azure-workload",
					IdentityID: pointers.Ptr("some-client-id"),
				},
			},
		},
		{
			name: "AzureEventHub with Workload identity with minimum properties",
			builder: utils.NewHorizontalScalingBuilder().WithAzureEventHubTrigger("some-client-id", &radixv1.RadixHorizontalScalingAzureEventHubTrigger{
				EventHubNamespace: "some-eventhub-namespace",
				EventHubName:      "some-eventhub-name",
				StorageAccount:    "some-storage-account",
				Container:         "some-container",
			}),
			expected: v1alpha1.ScaleTriggers{
				Name: "azure-event-hub",
				Type: "azure-eventhub",
				Metadata: map[string]string{
					"eventHubNamespace":  "some-eventhub-namespace",
					"eventHubName":       "some-eventhub-name",
					"storageAccountName": "some-storage-account",
					"blobContainer":      "some-container",
					"checkpointStrategy": "blobMetadata",
				},
				AuthenticationRef: &v1alpha1.AuthenticationRef{Name: utils.GetTriggerAuthenticationName(componentName, "azure-event-hub"), Kind: "TriggerAuthentication"},
			},
			expecedAuth: &v1alpha1.TriggerAuthenticationSpec{
				PodIdentity: &v1alpha1.AuthPodIdentity{
					Provider:   "azure-workload",
					IdentityID: pointers.Ptr("some-client-id"),
				},
			},
		},
		{
			name: "AzureEventHub with connection strings",
			builder: utils.NewHorizontalScalingBuilder().WithAzureEventHubTrigger("", &radixv1.RadixHorizontalScalingAzureEventHubTrigger{
				EventHubConnectionFromEnv:           "EVENTHUB_CONNECTION",
				EventHubName:                        "some-eventhub-name",
				ConsumerGroup:                       "some-consumer-group",
				StorageConnectionFromEnv:            "STORAGE_CONNECTION",
				Container:                           "some-container",
				CheckpointStrategy:                  "goSdk",
				UnprocessedEventThreshold:           pointers.Ptr(100),
				ActivationUnprocessedEventThreshold: pointers.Ptr(2),
			}),
			expected: v1alpha1.ScaleTriggers{
				Name: "azure-event-hub",
				Type: "azure-eventhub",
				Metadata: map[string]string{
					"connectionFromEnv":                   "EVENTHUB_CONNECTION",
					"eventHubName":                        "some-eventhub-name",
					"consumerGroup":                       "some-consumer-group",
					"storageConnectionFromEnv":            "STORAGE_CONNECTION",
					"checkpointStrategy":                  "goSdk",
					"blobContainer":                       "some-container",
					"unprocessedEventThreshold":           "100",
					"activationUnprocessedEventThreshold": "2",
				},
				AuthenticationRef: nil,
			},
			expecedAuth: nil,
		},
		{
			name: "AzureEventHub with connection strings with minimum properties",
			builder: utils.NewHorizontalScalingBuilder().WithAzureEventHubTrigger("", &radixv1.RadixHorizontalScalingAzureEventHubTrigger{
				EventHubConnectionFromEnv: "EVENTHUB_CONNECTION",
				StorageConnectionFromEnv:  "STORAGE_CONNECTION",
				Container:                 "some-container",
			}),
			expected: v1alpha1.ScaleTriggers{
				Name: "azure-event-hub",
				Type: "azure-eventhub",
				Metadata: map[string]string{
					"connectionFromEnv":        "EVENTHUB_CONNECTION",
					"storageConnectionFromEnv": "STORAGE_CONNECTION",
					"checkpointStrategy":       "blobMetadata",
					"blobContainer":            "some-container",
				},
				AuthenticationRef: nil,
			},
			expecedAuth: nil,
		},
	}

	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			// Test that memory scaling is not enabled by default
			rdBuilder := utils.ARadixDeployment().
				WithRadixApplication(raBuilder).
				WithComponents(utils.NewDeployComponentBuilder().
					WithName(componentName).
					WithHorizontalScaling(
						testcase.builder.
							WithMinReplicas(1).
							WithMaxReplicas(3).
							WithPollingInterval(4).
							WithCooldownPeriod(5).Build(),
					))

			rd, err := deployment.ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, rdBuilder)
			assert.NoError(t, err)

			scaler, err := kedaClient.KedaV1alpha1().ScaledObjects(rd.GetNamespace()).Get(context.Background(), rd.Spec.Components[0].GetName(), metav1.GetOptions{})
			require.NoError(t, err)
			require.Len(t, scaler.Spec.Triggers, 1, "Expected exactly one trigger in ScaledObject")
			trigger := scaler.Spec.Triggers[0]

			assert.Equal(t, testcase.expected, trigger, "Trigger spec does not match expected")

			if testcase.expecedAuth == nil {
				assert.Nil(t, testcase.expecedAuth, "Expected authentication spec should be nil for this test case")
			} else {
				actualAuth, err := kedaClient.KedaV1alpha1().TriggerAuthentications(rd.GetNamespace()).Get(context.Background(), testcase.expected.AuthenticationRef.Name, metav1.GetOptions{})
				require.NoError(t, err)

				assert.Equal(t, *testcase.expecedAuth, actualAuth.Spec, "Trigger authentication spec does not match expected")
			}

		})
	}
}

func TestScaledObjectSpec(t *testing.T) {
	tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := deployment.SetupTest(t)
	componentName := "a-component-name"
	rrBuilder := utils.ARadixRegistration().WithName("someapp")
	raBuilder := utils.ARadixApplication().
		WithRadixRegistration(rrBuilder)

	rdBuilder := utils.ARadixDeployment().
		WithRadixApplication(raBuilder).
		WithComponents(utils.NewDeployComponentBuilder().
			WithName(componentName).
			WithHorizontalScaling(
				utils.NewHorizontalScalingBuilder().
					WithCPUTrigger(80).
					WithMinReplicas(2).
					WithMaxReplicas(3).
					WithPollingInterval(4).
					WithCooldownPeriod(5).Build(),
			))

	rd, err := deployment.ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, rdBuilder)
	require.NoError(t, err)

	scaler, err := kedaClient.KedaV1alpha1().ScaledObjects(rd.GetNamespace()).Get(context.Background(), rd.Spec.Components[0].GetName(), metav1.GetOptions{})
	require.NoError(t, err)

	assert.Equal(t, int32(2), *scaler.Spec.MinReplicaCount)
	assert.Equal(t, int32(3), *scaler.Spec.MaxReplicaCount)
	assert.Equal(t, int32(4), *scaler.Spec.PollingInterval)
	assert.Equal(t, int32(5), *scaler.Spec.CooldownPeriod)

	require.Len(t, scaler.Spec.Triggers, 1)
	trigger := scaler.Spec.Triggers[0]
	assert.Equal(t, "cpu", trigger.Type)
	assert.Equal(t, "cpu", trigger.Name)
	assert.Equal(t, autoscalingv2.UtilizationMetricType, trigger.MetricType)
	assert.Equal(t, "80", trigger.Metadata["value"])

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
