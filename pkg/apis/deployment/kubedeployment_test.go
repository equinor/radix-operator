package deployment

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	type testScenario struct {
		command []string
		args    []string
	}
	testScenarios := map[string]testScenario{
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

	for testScenarioName, ts := range testScenarios {
		builderScenarios := []struct {
			name         string
			builder      utils.DeploymentBuilder
			getComponent func(rd *v1.RadixDeployment, name string) v1.RadixCommonDeployComponent
		}{
			{
				name: "component",
				builder: utils.ARadixDeployment().WithAppName("any-app").WithEnvironment("test").
					WithComponents(utils.NewDeployComponentBuilder().WithName("any-name-123").WithCommand(ts.command).WithArgs(ts.args)),
				getComponent: func(rd *v1.RadixDeployment, name string) v1.RadixCommonDeployComponent {
					return rd.GetComponentByName(name)
				},
			},
			{
				name: "job component",
				builder: utils.ARadixDeployment().WithAppName("any-app").WithEnvironment("test").
					WithJobComponents(utils.NewDeployJobComponentBuilder().WithName("any-name-123").WithCommand(ts.command).WithArgs(ts.args)),
				getComponent: func(rd *v1.RadixDeployment, name string) v1.RadixCommonDeployComponent {
					return rd.GetJobComponentByName(name)
				},
			},
		}
		for _, builderScenario := range builderScenarios {

			t.Run(fmt.Sprintf("%s: %s", builderScenario.name, testScenarioName), func(t *testing.T) {
				tu, client, kubeUtil, radixclient, kedaClient, prometheusClient, _, certClient := SetupTest(t)
				rd, _ := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusClient, certClient, builderScenario.builder)
				deployment := Deployment{radixclient: radixclient, kubeutil: kubeUtil, radixDeployment: rd}
				component := builderScenario.getComponent(rd, "any-name-123")
				assert.Equal(t, ts.command, component.GetCommand(), "command should match in RadixDeployment")
				assert.Equal(t, ts.args, component.GetArgs(), "args should match in RadixDeployment")

				desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(context.Background(), component)
				container := desiredDeployment.Spec.Template.Spec.Containers[0]

				assert.Equal(t, ts.command, container.Command, "command in desired Kubernetes deployment should match in RadixDeployment")
				assert.Equal(t, ts.args, container.Args, "args in desired Kubernetes deployment should match in RadixDeployment")
			})
		}
	}
}

func applyDeploymentWithSyncWithComponentResources(t *testing.T, origRequests, origLimits map[string]string) Deployment {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	rd, _ := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient,
		utils.ARadixDeployment().
			WithComponents(utils.NewDeployComponentBuilder().
				WithName("comp1").
				WithResource(origRequests, origLimits)).
			WithAppName("any-app").
			WithEnvironment("test"))
	return Deployment{radixclient: radixclient, kubeutil: kubeUtil, radixDeployment: rd}
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
