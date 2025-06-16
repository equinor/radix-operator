package deployment

import (
	"context"
	"os"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func teardownReadinessProbe() {
	_ = os.Unsetenv(defaults.OperatorReadinessProbeInitialDelaySeconds)
	_ = os.Unsetenv(defaults.OperatorReadinessProbePeriodSeconds)
}

func TestGetReadinessProbe_MissingDefaultEnvVars(t *testing.T) {
	teardownReadinessProbe()

	probe, err := getDefaultReadinessProbeForComponent(&v1.RadixDeployComponent{Ports: []v1.ComponentPort{{Name: "http", Port: int32(80)}}})
	assert.Error(t, err)
	assert.Nil(t, probe)
}

func TestGetReadinessProbe_Custom(t *testing.T) {
	test.SetRequiredEnvironmentVariables()

	probe, err := getDefaultReadinessProbeForComponent(&v1.RadixDeployComponent{Ports: []v1.ComponentPort{{Name: "http", Port: int32(5000)}}})
	assert.Nil(t, err)

	assert.Equal(t, int32(5), probe.InitialDelaySeconds)
	assert.Equal(t, int32(10), probe.PeriodSeconds)
	assert.Equal(t, int32(5000), probe.ProbeHandler.TCPSocket.Port.IntVal)

	teardownReadinessProbe()
}
func TestComponentWithoutCustomHealthChecks(t *testing.T) {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	rd, _ := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient,
		utils.ARadixDeployment().
			WithComponents(utils.NewDeployComponentBuilder().
				WithName("comp1")).
			WithAppName("any-app").
			WithEnvironment("test"))

	component := rd.GetComponentByName("comp1")
	assert.Nil(t, component.HealthChecks)
}
func TestComponentWithCustomHealthChecks(t *testing.T) {
	tu, kubeClient, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	createProbe := func(handler v1.RadixProbeHandler, seconds int32) *v1.RadixProbe {
		return &v1.RadixProbe{
			RadixProbeHandler:   handler,
			InitialDelaySeconds: seconds,
			TimeoutSeconds:      seconds + 1,
			PeriodSeconds:       seconds + 2,
			SuccessThreshold:    seconds + 3,
			FailureThreshold:    seconds + 4,
			// TerminationGracePeriodSeconds: pointers.Ptr(int64(seconds + 5)),
		}
	}

	readynessProbe := createProbe(v1.RadixProbeHandler{HTTPGet: &v1.RadixProbeHTTPGetAction{
		Port: 5000,
	}}, 10)

	livenessProbe := createProbe(v1.RadixProbeHandler{TCPSocket: &v1.RadixProbeTCPSocketAction{
		Port: 5000,
	}}, 20)
	startupProbe := createProbe(v1.RadixProbeHandler{Exec: &v1.RadixProbeExecAction{
		Command: []string{"echo", "hello"},
	}}, 30)

	rd, err := ApplyDeploymentWithSync(tu, kubeClient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient,
		utils.ARadixDeployment().
			WithComponents(utils.NewDeployComponentBuilder().
				WithName("comp1").
				WithHealthChecks(startupProbe, readynessProbe, livenessProbe)).
			WithAppName("any-app").
			WithEnvironment("test").
			WithDeploymentName("deployment1"))
	require.NoError(t, err, "failed to apply deployment1")

	component := rd.GetComponentByName("comp1")
	require.NotNil(t, component.HealthChecks)
	assert.Equal(t, readynessProbe, component.HealthChecks.ReadinessProbe)
	assert.Equal(t, livenessProbe, component.HealthChecks.LivenessProbe)
	assert.Equal(t, startupProbe, component.HealthChecks.StartupProbe)
	deployment, err := kubeClient.AppsV1().Deployments("any-app-test").Get(context.Background(), "comp1", metav1.GetOptions{})
	require.NoError(t, err, "failed to get deployment")
	assert.NotNil(t, deployment.Spec.Template.Spec.Containers[0].ReadinessProbe, "readiness probe should be set")
	assert.NotNil(t, deployment.Spec.Template.Spec.Containers[0].LivenessProbe, "liveness probe should be set")
	assert.NotNil(t, deployment.Spec.Template.Spec.Containers[0].StartupProbe, "startup probe should be set")
	assert.Equal(t, readynessProbe.InitialDelaySeconds, deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.InitialDelaySeconds, "invalid readiness probe initial delay")
	assert.Equal(t, livenessProbe.InitialDelaySeconds, deployment.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds, "invalid liveness probe initial delay")
	assert.Equal(t, startupProbe.InitialDelaySeconds, deployment.Spec.Template.Spec.Containers[0].StartupProbe.InitialDelaySeconds, "invalid startup probe initial delay")

	rd, err = ApplyDeploymentWithSync(tu, kubeClient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient,
		utils.ARadixDeployment().
			WithComponents(utils.NewDeployComponentBuilder().
				WithName("comp1").WithPorts([]v1.ComponentPort{{Name: "http", Port: 8000}}).WithPublicPort("http")).
			WithAppName("any-app").
			WithEnvironment("test").
			WithDeploymentName("deployment2"))
	require.NoError(t, err, "failed to apply deployment")

	component = rd.GetComponentByName("comp1")
	require.Nil(t, component.HealthChecks)
	deployment, err = kubeClient.AppsV1().Deployments("any-app-test").Get(context.Background(), "comp1", metav1.GetOptions{})
	require.NoError(t, err, "failed to get deployment2")
	assert.NotNil(t, deployment.Spec.Template.Spec.Containers[0].ReadinessProbe, "default readiness probe should be set")
	assert.Nil(t, deployment.Spec.Template.Spec.Containers[0].LivenessProbe, "liveness probe should not be set")
	assert.Nil(t, deployment.Spec.Template.Spec.Containers[0].StartupProbe, "startup probe should not be set")
	assert.Equal(t, int32(5), deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.InitialDelaySeconds, "invalid default readiness probe initial delay")
	assert.NotNil(t, deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.TCPSocket, "default readiness probe should be tcp")
	assert.Equal(t, int32(8000), deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.TCPSocket.Port.IntVal, "invalid default readiness probe port")
}

func Test_Sync_AppID_OnNewDeployment(t *testing.T) {
	appId := ulid.Make()
	tu, kubeClient, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	_, err := ApplyDeploymentWithSync(tu, kubeClient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient,
		utils.ARadixDeployment().
			WithRadixApplication(utils.ARadixApplication().WithRadixRegistration(utils.ARadixRegistration().WithAppID(appId))).
			WithComponents(utils.NewDeployComponentBuilder().
				WithName("comp1")).
			WithAppName("any-app").
			WithEnvironment("test").
			WithDeploymentName("deployment1"))
	require.NoError(t, err, "failed to apply deployment1")
	deployment, err := kubeClient.AppsV1().Deployments("any-app-test").Get(context.Background(), "comp1", metav1.GetOptions{})
	require.NoError(t, err, "failed to apply deployment1")

	assert.Equal(t, deployment.Spec.Template.Labels["radix-app-id"], appId.String(), "radix-id label should be set")
}

func Test_DoNot_SyncAppID_WhenMissing(t *testing.T) {
	tu, kubeClient, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	_, err := ApplyDeploymentWithSync(tu, kubeClient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient,
		utils.ARadixDeployment().
			WithRadixApplication(utils.ARadixApplication().WithRadixRegistration(utils.ARadixRegistration().WithAppID(ulid.Zero))).
			WithComponents(utils.NewDeployComponentBuilder().
				WithName("comp1")).
			WithAppName("any-app").
			WithEnvironment("test").
			WithDeploymentName("deployment1"))
	require.NoError(t, err, "failed to apply deployment1")
	deployment, err := kubeClient.AppsV1().Deployments("any-app-test").Get(context.Background(), "comp1", metav1.GetOptions{})
	require.NoError(t, err, "failed to apply deployment1")

	assert.NotContains(t, deployment.Spec.Template.Labels, "radix-app-id", "radix-id label should be set")
}

func Test_DoNot_SyncAppID_WhenAddedLater(t *testing.T) {
	tu, kubeClient, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)

	rrBuilder := utils.ARadixRegistration().WithAppID(ulid.Zero).WithName("any-app")
	rdBuilder := utils.ARadixDeployment().
		WithAppName("any-app").
		WithRadixApplication(utils.ARadixApplication().WithRadixRegistration(rrBuilder)).
		WithComponents(utils.NewDeployComponentBuilder().
			WithName("comp1")).
		WithEnvironment("test").
		WithDeploymentName("deployment1")

	rd, err := ApplyDeploymentWithSync(tu, kubeClient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, rdBuilder)
	require.NoError(t, err, "failed to apply deployment1")

	rr, err := radixclient.RadixV1().RadixRegistrations().Get(context.Background(), rd.Spec.AppName, metav1.GetOptions{})
	require.NoError(t, err)

	rr.Spec.AppID = ulid.Make() // Set a new app ID
	_, err = radixclient.RadixV1().RadixRegistrations().Update(context.Background(), rr, metav1.UpdateOptions{})
	require.NoError(t, err)

	applyDeploymentUpdateWithSync(tu, kubeClient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, rdBuilder)

	deployment, err := kubeClient.AppsV1().Deployments("any-app-test").Get(context.Background(), "comp1", metav1.GetOptions{})
	require.NoError(t, err, "failed to apply deployment1")

	assert.NotContains(t, deployment.Spec.Template.Labels, "radix-app-id", "radix-id label should be set")
}

func Test_UpdateResourcesInDeployment(t *testing.T) {
	origRequests := map[string]string{"cpu": "10m", "memory": "100M"}
	origLimits := map[string]string{"cpu": "100m", "memory": "1000M"}

	t.Run("set empty requests and limits", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(t, nil, nil)

		expectedRequests := map[string]string{"cpu": "20m", "memory": "200M"}
		expectedLimits := map[string]string{"cpu": "30m", "memory": "300M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, expectedLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(context.Background(), &component)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, parseQuantity(expectedRequests["cpu"]), desiredRes.Requests["cpu"])
		assert.Equal(t, parseQuantity(expectedRequests["memory"]), desiredRes.Requests["memory"])
		assert.Equal(t, parseQuantity(expectedLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(expectedLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("set empty requests and update limit", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(t, nil, origLimits)

		expectedRequests := map[string]string{"cpu": "20m", "memory": "200M"}
		expectedLimits := map[string]string{"cpu": "30m", "memory": "300M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, expectedLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(context.Background(), &component)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, parseQuantity(expectedRequests["cpu"]), desiredRes.Requests["cpu"])
		assert.Equal(t, parseQuantity(expectedRequests["memory"]), desiredRes.Requests["memory"])
		assert.Equal(t, parseQuantity(expectedLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(expectedLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("update requests and set empty limits", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(t, origRequests, nil)

		expectedRequests := map[string]string{"cpu": "20m", "memory": "200M"}
		expectedLimits := map[string]string{"cpu": "30m", "memory": "300M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, expectedLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(context.Background(), &component)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, parseQuantity(expectedRequests["cpu"]), desiredRes.Requests["cpu"])
		assert.Equal(t, parseQuantity(expectedRequests["memory"]), desiredRes.Requests["memory"])
		assert.Equal(t, parseQuantity(expectedLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(expectedLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("update requests and limits", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(t, origRequests, origLimits)

		expectedRequests := map[string]string{"cpu": "20m", "memory": "200M"}
		expectedLimits := map[string]string{"cpu": "30m", "memory": "300M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, expectedLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(context.Background(), &component)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, parseQuantity(expectedRequests["cpu"]), desiredRes.Requests["cpu"])
		assert.Equal(t, parseQuantity(expectedRequests["memory"]), desiredRes.Requests["memory"])
		assert.Equal(t, parseQuantity(expectedLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(expectedLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("update requests memory without cpu", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(t, origRequests, origLimits)

		expectedRequests := map[string]string{"memory": "200M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, origLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(context.Background(), &component)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.NotContains(t, desiredRes.Requests, "cpu")
		assert.Equal(t, parseQuantity(expectedRequests["memory"]), desiredRes.Requests["memory"])
		assert.Equal(t, parseQuantity(origLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(origLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("update requests cpu without memory", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(t, origRequests, origLimits)

		expectedRequests := map[string]string{"cpu": "20m"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, origLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(context.Background(), &component)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, parseQuantity(expectedRequests["cpu"]), desiredRes.Requests["cpu"])
		assert.NotContains(t, desiredRes.Requests, "memory")
		assert.Equal(t, parseQuantity(origLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(origLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("keep requests and update limits", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(t, origRequests, origLimits)

		expectedLimits := map[string]string{"cpu": "30m", "memory": "300M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(origRequests, expectedLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(context.Background(), &component)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, parseQuantity(origRequests["cpu"]), desiredRes.Requests["cpu"])
		assert.Equal(t, parseQuantity(origRequests["memory"]), desiredRes.Requests["memory"])
		assert.Equal(t, parseQuantity(expectedLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(expectedLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("update requests and keep limits", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(t, origRequests, origLimits)

		expectedRequests := map[string]string{"cpu": "20m", "memory": "200M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, origLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(context.Background(), &component)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, parseQuantity(expectedRequests["cpu"]), desiredRes.Requests["cpu"])
		assert.Equal(t, parseQuantity(expectedRequests["memory"]), desiredRes.Requests["memory"])
		assert.Equal(t, parseQuantity(origLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(origLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("remove requests and limits", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(t, origRequests, origLimits)

		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(nil, nil).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(context.Background(), &component)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, 0, len(desiredRes.Requests))
		assert.Equal(t, 0, len(desiredRes.Limits))
	})
	t.Run("update requests and remove limit", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(t, origRequests, origLimits)

		expectedRequests := map[string]string{"cpu": "20m", "memory": "200M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, nil).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(context.Background(), &component)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, expectedRequests["cpu"], pointers.Ptr(desiredRes.Requests["cpu"]).String())
		assert.Equal(t, expectedRequests["memory"], pointers.Ptr(desiredRes.Requests["memory"]).String())
		assert.Equal(t, 0, len(desiredRes.Limits))
	})
	t.Run("remove requests and update limit", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(t, origRequests, origLimits)

		var expectedRequests map[string]string
		expectedLimits := map[string]string{"cpu": "30m", "memory": "300M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, expectedLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(context.Background(), &component)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, 0, len(desiredRes.Requests))
		assert.Equal(t, parseQuantity(expectedLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(expectedLimits["memory"]), desiredRes.Limits["memory"])
	})
}

func Test_CommandAndArgs(t *testing.T) {
	type scenario struct {
		command []string
		args    []string
	}
	scenarios := map[string]scenario{
		"command and args are not set": {
			command: nil,
			args:    nil,
		},
		"single command is set": {
			command: []string{"bash"},
			args:    nil,
		},
		"command with arguments is set": {
			command: []string{"sh", "-c", "echo hello"},
			args:    nil,
		},
		"command is set and args are set": {
			command: []string{"sh", "-c"},
			args:    []string{"echo hello"},
		},
		"only args are set": {
			command: nil,
			args:    []string{"--verbose", "--output=json"},
		},
	}

	for name, ts := range scenarios {
		t.Run(name, func(t *testing.T) {
			tu, kubeClient, kubeUtil, radixclient, kedaClient, prometheusClient, _, certClient := SetupTest(t)
			builder := utils.ARadixDeployment().WithAppName("any-app").WithEnvironment("test").
				WithComponents(
					utils.NewDeployComponentBuilder().WithName("comp1").WithCommand(ts.command).WithArgs(ts.args),
					utils.NewDeployComponentBuilder().WithName("comp2"),
				).
				WithJobComponents(
					utils.NewDeployJobComponentBuilder().WithName("job1").WithCommand(ts.command).WithArgs(ts.args),
					utils.NewDeployJobComponentBuilder().WithName("job2"),
				)
			rd, _ := ApplyDeploymentWithSync(tu, kubeClient, kubeUtil, radixclient, kedaClient, prometheusClient, certClient, builder)

			component1 := rd.GetComponentByName("comp1")
			assert.Equal(t, ts.command, component1.GetCommand(), "command in component1 should match in RadixDeployment")
			assert.Equal(t, ts.args, component1.GetArgs(), "args in component1 should match in RadixDeployment")

			component2 := rd.GetComponentByName("comp2")
			assert.Empty(t, component2.GetCommand(), "command in component2 should be empty")
			assert.Empty(t, component2.GetArgs(), "args in component2 should be empty")

			job1 := rd.GetJobComponentByName("job1")
			assert.Equal(t, ts.command, job1.GetCommand(), "command in job 1 should match in RadixDeployment")
			assert.Equal(t, ts.args, job1.GetArgs(), "args in job 1 should match in RadixDeployment")

			job2 := rd.GetJobComponentByName("job2")
			assert.Empty(t, job2.GetCommand(), "command in job 2 should be empty")
			assert.Empty(t, job2.GetArgs(), "args in job 2 should be empty")

			deploymentList, err := kubeClient.AppsV1().Deployments(utils.GetEnvironmentNamespace(rd.Spec.AppName, rd.Spec.Environment)).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err, "failed to list deployments")
			deployments := slice.Reduce(deploymentList.Items, make(map[string]*appsv1.Deployment, len(deploymentList.Items)), func(acc map[string]*appsv1.Deployment, depl appsv1.Deployment) map[string]*appsv1.Deployment {
				acc[depl.Name] = &depl
				return acc
			})

			desiredDeploymentComp1, ok := deployments[component1.Name]
			if assert.True(t, ok, "deployment component1 should exist") {
				assert.Equal(t, ts.command, desiredDeploymentComp1.Spec.Template.Spec.Containers[0].Command, "command in desired Kubernetes deployment for component1 should match in RadixDeployment")
				assert.Equal(t, ts.args, desiredDeploymentComp1.Spec.Template.Spec.Containers[0].Args, "args in desired Kubernetes deployment for component1 should match in RadixDeployment")
			}

			desiredDeploymentComp2, ok := deployments[component2.Name]
			if assert.True(t, ok, "deployment component2 should exist") {
				assert.Empty(t, desiredDeploymentComp2.Spec.Template.Spec.Containers[0].Command, "command in desired Kubernetes deployment for component2 should be empty")
				assert.Empty(t, desiredDeploymentComp2.Spec.Template.Spec.Containers[0].Args, "args in desired Kubernetes deployment should for component2 should be empty")
			}

			desiredDeploymentJob1, ok := deployments[job1.Name]
			if assert.True(t, ok, "deployment job1 should exist") {
				assert.Empty(t, desiredDeploymentJob1.Spec.Template.Spec.Containers[0].Command, "command in desired Kubernetes deployment for job1 should match in RadixDeployment")
				assert.Empty(t, desiredDeploymentJob1.Spec.Template.Spec.Containers[0].Args, "args in desired Kubernetes deployment for job1 should match in RadixDeployment")
			}

			desiredDeploymentJob2, ok := deployments[job2.Name]
			if assert.True(t, ok, "deployment job2 should exist") {
				assert.Empty(t, desiredDeploymentJob2.Spec.Template.Spec.Containers[0].Command, "command in desired Kubernetes deployment for job2 should be empty")
				assert.Empty(t, desiredDeploymentJob2.Spec.Template.Spec.Containers[0].Args, "args in desired Kubernetes deployment should for job2 should be empty")
			}
		})
	}
}

func applyDeploymentWithSyncWithComponentResources(t *testing.T, origRequests, origLimits map[string]string) Deployment {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)

	rr := utils.ARadixRegistration()
	ra := utils.ARadixApplication().WithRadixRegistration(rr)

	rd, _ := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient,
		utils.ARadixDeployment().WithRadixApplication(ra).
			WithComponents(utils.NewDeployComponentBuilder().
				WithName("comp1").
				WithResource(origRequests, origLimits)).
			WithAppName("any-app").
			WithEnvironment("test"))
	return Deployment{radixclient: radixclient, kubeutil: kubeUtil, radixDeployment: rd, registration: rr.BuildRR()}
}

func TestDeployment_createJobAuxDeployment(t *testing.T) {
	deploy := &Deployment{radixDeployment: &v1.RadixDeployment{ObjectMeta: metav1.ObjectMeta{Name: "deployment1", UID: "uid1"}}}
	jobAuxDeployment := deploy.createJobAuxDeployment("job1", "job1-aux")
	assert.Equal(t, "job1-aux", jobAuxDeployment.GetName())
	resources := jobAuxDeployment.Spec.Template.Spec.Containers[0].Resources
	s := resources.Requests.Cpu().String()
	assert.Equal(t, "1m", s)
	assert.Equal(t, "20M", resources.Requests.Memory().String())
	assert.Equal(t, "0", resources.Limits.Cpu().String())
	assert.Equal(t, "20M", resources.Limits.Memory().String())
}
