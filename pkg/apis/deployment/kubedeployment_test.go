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
