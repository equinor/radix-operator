package deployment

import (
	"os"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func teardownRollingUpdate() {
	os.Unsetenv(defaults.OperatorRollingUpdateMaxUnavailable)
	os.Unsetenv(defaults.OperatorRollingUpdateMaxSurge)
}

func teardownReadinessProbe() {
	os.Unsetenv(defaults.OperatorReadinessProbeInitialDelaySeconds)
	os.Unsetenv(defaults.OperatorReadinessProbePeriodSeconds)
}

func TestGetResourceRequirements_BothProvided_BothReturned(t *testing.T) {
	test.SetRequiredEnvironmentVariables()

	request := map[string]string{
		"cpu":    "0.1",
		"memory": "32Mi",
	}

	limit := map[string]string{
		"cpu":    "0.5",
		"memory": "64Mi",
	}

	component := utils.NewDeployComponentBuilder().
		WithResource(request, limit).
		BuildComponent()
	requirements := utils.GetResourceRequirements(&component)

	assert.Equal(t, 0, requirements.Requests.Cpu().Cmp(resource.MustParse("0.1")), "CPU request should be included")
	assert.Equal(t, 0, requirements.Requests.Memory().Cmp(resource.MustParse("32Mi")), "Memory request should be included")

	assert.Equal(t, 0, requirements.Limits.Cpu().Cmp(resource.MustParse("0.5")), "CPU limit should be included")
	assert.Equal(t, 0, requirements.Limits.Memory().Cmp(resource.MustParse("64Mi")), "Memory limit should be included")
}

func TestGetResourceRequirements_ProvideRequests_OnlyRequestsReturned(t *testing.T) {
	test.SetRequiredEnvironmentVariables()

	request := map[string]string{
		"cpu":    "0.2",
		"memory": "128Mi",
	}

	component := utils.NewDeployComponentBuilder().
		WithResourceRequestsOnly(request).
		BuildComponent()
	requirements := utils.GetResourceRequirements(&component)

	assert.Equal(t, 0, requirements.Requests.Cpu().Cmp(resource.MustParse("0.2")), "CPU request should be included")
	assert.Equal(t, 0, requirements.Requests.Memory().Cmp(resource.MustParse("128Mi")), "Memory request should be included")

	assert.Equal(t, 0, requirements.Limits.Cpu().Cmp(resource.MustParse("0")), "Missing CPU limit should be 0")
	assert.Equal(t, 0, requirements.Limits.Memory().Cmp(resource.MustParse("0")), "Missing memory limit should be 0")
}

func TestGetResourceRequirements_ProvideRequestsCpu_OnlyRequestsCpuReturned(t *testing.T) {
	test.SetRequiredEnvironmentVariables()

	request := map[string]string{
		"cpu": "0.3",
	}

	component := utils.NewDeployComponentBuilder().
		WithResourceRequestsOnly(request).
		BuildComponent()
	requirements := utils.GetResourceRequirements(&component)

	assert.Equal(t, 0, requirements.Requests.Cpu().Cmp(resource.MustParse("0.3")), "CPU request should be included")
	assert.Equal(t, 0, requirements.Requests.Memory().Cmp(resource.MustParse("0")), "Missing memory request should be 0")

	assert.Equal(t, 0, requirements.Limits.Cpu().Cmp(resource.MustParse("0")), "Missing CPU limit should be 0")
	assert.Equal(t, 0, requirements.Limits.Memory().Cmp(resource.MustParse("0")), "Missing memory limit should be 0")
}

func TestGetResourceRequirements_BothProvided_OverDefaultLimits(t *testing.T) {
	test.SetRequiredEnvironmentVariables()

	request := map[string]string{
		"cpu":    "5",
		"memory": "5Gi",
	}

	component := utils.NewDeployComponentBuilder().
		WithResourceRequestsOnly(request).
		BuildComponent()
	requirements := utils.GetResourceRequirements(&component)

	assert.Equal(t, 0, requirements.Requests.Cpu().Cmp(resource.MustParse("5")), "CPU request should be included")
	assert.Equal(t, 0, requirements.Requests.Memory().Cmp(resource.MustParse("5Gi")), "Memory request should be included")

	assert.Equal(t, 0, requirements.Limits.Cpu().Cmp(resource.MustParse("5")), "CPU limit should be same as request")
	assert.Equal(t, 0, requirements.Limits.Memory().Cmp(resource.MustParse("5Gi")), "Memory limit should be same as request")
}

func TestGetResourceRequirements_ProvideRequestsCpu_OverDefaultLimits(t *testing.T) {
	test.SetRequiredEnvironmentVariables()

	request := map[string]string{
		"cpu": "6",
	}

	component := utils.NewDeployComponentBuilder().
		WithResourceRequestsOnly(request).
		BuildComponent()
	requirements := utils.GetResourceRequirements(&component)

	assert.Equal(t, 0, requirements.Requests.Cpu().Cmp(resource.MustParse("6")), "CPU request should be included")
	assert.Equal(t, 0, requirements.Requests.Memory().Cmp(resource.MustParse("0")), "Missing memory request should be 0")

	assert.Equal(t, 0, requirements.Limits.Cpu().Cmp(resource.MustParse("6")), "CPU limit should be same as request")
	assert.Equal(t, 0, requirements.Limits.Memory().Cmp(resource.MustParse("0")), "Missing memory limit should be 0")
}

func TestGetReadinessProbe_Default(t *testing.T) {
	teardownReadinessProbe()

	probe := corev1.Probe{}
	componentPort := v1.ComponentPort{Port: int32(80)}

	err := getReadinessProbeSettings(&probe, &componentPort)

	assert.NotNil(t, err)
}

func TestGetReadinessProbe_Custom(t *testing.T) {
	test.SetRequiredEnvironmentVariables()

	probe := corev1.Probe{
		Handler: corev1.Handler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.IntOrString{
					IntVal: int32(0),
				},
			},
		},
		InitialDelaySeconds: int32(0),
		PeriodSeconds:       int32(0),
	}
	componentPort := v1.ComponentPort{Port: int32(80)}
	getReadinessProbeSettings(&probe, &componentPort)

	assert.Equal(t, int32(5), probe.InitialDelaySeconds)
	assert.Equal(t, int32(10), probe.PeriodSeconds)
	assert.Equal(t, componentPort.Port, probe.Handler.TCPSocket.Port.IntVal)

	teardownReadinessProbe()
}

func Test_UpdateResourcesInDeployment(t *testing.T) {
	origRequests := map[string]string{"cpu": "10mi", "memory": "100M"}
	origLimits := map[string]string{"cpu": "100mi", "memory": "1000M"}
	envVarsConfigMap := &corev1.ConfigMap{Data: map[string]string{}}
	envVarsMetadata := map[string]v1.EnvVarMetadata{}

	t.Run("set empty requests and limits", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(nil, nil)

		expectedRequests := map[string]string{"cpu": "20mi", "memory": "200M"}
		expectedLimits := map[string]string{"cpu": "30mi", "memory": "300M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, expectedLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(&component, envVarsConfigMap, envVarsMetadata)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, parseQuantity(expectedRequests["cpu"]), desiredRes.Requests["cpu"])
		assert.Equal(t, parseQuantity(expectedRequests["memory"]), desiredRes.Requests["memory"])
		assert.Equal(t, parseQuantity(expectedLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(expectedLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("set empty requests and update limit", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(nil, origLimits)

		expectedRequests := map[string]string{"cpu": "20mi", "memory": "200M"}
		expectedLimits := map[string]string{"cpu": "30mi", "memory": "300M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, expectedLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(&component, envVarsConfigMap, envVarsMetadata)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, parseQuantity(expectedRequests["cpu"]), desiredRes.Requests["cpu"])
		assert.Equal(t, parseQuantity(expectedRequests["memory"]), desiredRes.Requests["memory"])
		assert.Equal(t, parseQuantity(expectedLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(expectedLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("update requests and set empty limits", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(origRequests, nil)

		expectedRequests := map[string]string{"cpu": "20mi", "memory": "200M"}
		expectedLimits := map[string]string{"cpu": "30mi", "memory": "300M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, expectedLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(&component, envVarsConfigMap, envVarsMetadata)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, parseQuantity(expectedRequests["cpu"]), desiredRes.Requests["cpu"])
		assert.Equal(t, parseQuantity(expectedRequests["memory"]), desiredRes.Requests["memory"])
		assert.Equal(t, parseQuantity(expectedLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(expectedLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("update requests and limits", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(origRequests, origLimits)

		expectedRequests := map[string]string{"cpu": "20mi", "memory": "200M"}
		expectedLimits := map[string]string{"cpu": "30mi", "memory": "300M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, expectedLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(&component, envVarsConfigMap, envVarsMetadata)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, parseQuantity(expectedRequests["cpu"]), desiredRes.Requests["cpu"])
		assert.Equal(t, parseQuantity(expectedRequests["memory"]), desiredRes.Requests["memory"])
		assert.Equal(t, parseQuantity(expectedLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(expectedLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("update requests memory without cpu", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(origRequests, origLimits)

		expectedRequests := map[string]string{"memory": "200M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, origLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(&component, envVarsConfigMap, envVarsMetadata)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.NotContains(t, desiredRes.Requests, "cpu")
		assert.Equal(t, parseQuantity(expectedRequests["memory"]), desiredRes.Requests["memory"])
		assert.Equal(t, parseQuantity(origLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(origLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("update requests cpu without memory", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(origRequests, origLimits)

		expectedRequests := map[string]string{"cpu": "20mi"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, origLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(&component, envVarsConfigMap, envVarsMetadata)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, parseQuantity(expectedRequests["cpu"]), desiredRes.Requests["cpu"])
		assert.NotContains(t, desiredRes.Requests, "memory")
		assert.Equal(t, parseQuantity(origLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(origLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("keep requests and update limits", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(origRequests, origLimits)

		expectedLimits := map[string]string{"cpu": "30mi", "memory": "300M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(origRequests, expectedLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(&component, envVarsConfigMap, envVarsMetadata)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, parseQuantity(origRequests["cpu"]), desiredRes.Requests["cpu"])
		assert.Equal(t, parseQuantity(origRequests["memory"]), desiredRes.Requests["memory"])
		assert.Equal(t, parseQuantity(expectedLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(expectedLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("update requests and keep limits", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(origRequests, origLimits)

		expectedRequests := map[string]string{"cpu": "20mi", "memory": "200M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, origLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(&component, envVarsConfigMap, envVarsMetadata)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, parseQuantity(expectedRequests["cpu"]), desiredRes.Requests["cpu"])
		assert.Equal(t, parseQuantity(expectedRequests["memory"]), desiredRes.Requests["memory"])
		assert.Equal(t, parseQuantity(origLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(origLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("remove requests and limits", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(origRequests, origLimits)

		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(nil, nil).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(&component, envVarsConfigMap, envVarsMetadata)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, 0, len(desiredRes.Requests))
		assert.Equal(t, 0, len(desiredRes.Limits))
	})
	t.Run("update requests and remove limit", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(origRequests, origLimits)

		expectedRequests := map[string]string{"cpu": "20mi", "memory": "200M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, nil).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(&component, envVarsConfigMap, envVarsMetadata)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, parseQuantity(expectedRequests["cpu"]), desiredRes.Requests["cpu"])
		assert.Equal(t, parseQuantity(expectedRequests["memory"]), desiredRes.Requests["memory"])
		assert.Equal(t, 0, len(desiredRes.Limits))
	})
	t.Run("remove requests and update limit", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(origRequests, origLimits)

		var expectedRequests map[string]string
		expectedLimits := map[string]string{"cpu": "30mi", "memory": "300M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, expectedLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(&component, envVarsConfigMap, envVarsMetadata)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, 0, len(desiredRes.Requests))
		assert.Equal(t, parseQuantity(expectedLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(expectedLimits["memory"]), desiredRes.Limits["memory"])
	})
}

func TestSetDeploymentStrategy_Default(t *testing.T) {
	teardownRollingUpdate()
	deploymentStrategy := createDeploymentStrategy()
	err := setDeploymentStrategy(deploymentStrategy)
	assert.NotNil(t, err)
}

func TestSetDeploymentStrategy_Custom(t *testing.T) {
	test.SetRequiredEnvironmentVariables()

	deploymentStrategy := createDeploymentStrategy()
	setDeploymentStrategy(deploymentStrategy)

	assert.Equal(t, "25%", deploymentStrategy.RollingUpdate.MaxUnavailable.StrVal)
	assert.Equal(t, "25%", deploymentStrategy.RollingUpdate.MaxSurge.StrVal)

	teardownRollingUpdate()
}

func createDeploymentStrategy() *appsv1.DeploymentStrategy {
	deploymentStrategy := appsv1.DeploymentStrategy{
		RollingUpdate: &appsv1.RollingUpdateDeployment{
			MaxUnavailable: &intstr.IntOrString{
				Type:   intstr.String,
				StrVal: "none",
			},
			MaxSurge: &intstr.IntOrString{
				Type:   intstr.String,
				StrVal: "none",
			},
		},
	}
	return &deploymentStrategy
}

func applyDeploymentWithSyncWithComponentResources(origRequests, origLimits map[string]string) Deployment {
	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()
	rd, _ := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient,
		utils.ARadixDeployment().
			WithComponents(utils.NewDeployComponentBuilder().
				WithName("comp1").
				WithResource(origRequests, origLimits)).
			WithAppName("any-app").
			WithEnvironment("test"))
	return Deployment{radixclient: radixclient, kubeutil: kubeUtil, radixDeployment: rd}
}
