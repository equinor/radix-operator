package deployment

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestGetResourceRequirements_BothProvided_BothReturned(t *testing.T) {
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
	requirements := getResourceRequirements(component)

	assert.Equal(t, 0, requirements.Requests.Cpu().Cmp(resource.MustParse("0.1")), "CPU request should be included")
	assert.Equal(t, 0, requirements.Requests.Memory().Cmp(resource.MustParse("32Mi")), "Memory request should be included")

	assert.Equal(t, 0, requirements.Limits.Cpu().Cmp(resource.MustParse("0.5")), "CPU limit should be included")
	assert.Equal(t, 0, requirements.Limits.Memory().Cmp(resource.MustParse("64Mi")), "Memory limit should be included")
}

func TestGetResourceRequirements_ProvideRequests_OnlyRequestsReturned(t *testing.T) {
	request := map[string]string{
		"cpu":    "0.2",
		"memory": "128Mi",
	}

	component := utils.NewDeployComponentBuilder().
		WithResourceRequestsOnly(request).
		BuildComponent()
	requirements := getResourceRequirements(component)

	assert.Equal(t, 0, requirements.Requests.Cpu().Cmp(resource.MustParse("0.2")), "CPU request should be included")
	assert.Equal(t, 0, requirements.Requests.Memory().Cmp(resource.MustParse("128Mi")), "Memory request should be included")

	assert.Equal(t, 0, requirements.Limits.Cpu().Cmp(resource.MustParse("0")), "Missing CPU limit should be 0")
	assert.Equal(t, 0, requirements.Limits.Memory().Cmp(resource.MustParse("0")), "Missing memory limit should be 0")
}

func TestGetResourceRequirements_ProvideRequestsCpu_OnlyRequestsCpuReturned(t *testing.T) {
	request := map[string]string{
		"cpu": "0.3",
	}

	component := utils.NewDeployComponentBuilder().
		WithResourceRequestsOnly(request).
		BuildComponent()
	requirements := getResourceRequirements(component)

	assert.Equal(t, 0, requirements.Requests.Cpu().Cmp(resource.MustParse("0.3")), "CPU request should be included")
	assert.Equal(t, 0, requirements.Requests.Memory().Cmp(resource.MustParse("0")), "Missing memory request should be 0")

	assert.Equal(t, 0, requirements.Limits.Cpu().Cmp(resource.MustParse("0")), "Missing CPU limit should be 0")
	assert.Equal(t, 0, requirements.Limits.Memory().Cmp(resource.MustParse("0")), "Missing memory limit should be 0")
}

func TestGetResourceRequirements_BothProvided_OverDefaultLimits(t *testing.T) {
	request := map[string]string{
		"cpu":    "5",
		"memory": "5Gi",
	}

	component := utils.NewDeployComponentBuilder().
		WithResourceRequestsOnly(request).
		BuildComponent()
	requirements := getResourceRequirements(component)

	assert.Equal(t, 0, requirements.Requests.Cpu().Cmp(resource.MustParse("5")), "CPU request should be included")
	assert.Equal(t, 0, requirements.Requests.Memory().Cmp(resource.MustParse("5Gi")), "Memory request should be included")

	assert.Equal(t, 0, requirements.Limits.Cpu().Cmp(resource.MustParse("5")), "CPU limit should be same as request")
	assert.Equal(t, 0, requirements.Limits.Memory().Cmp(resource.MustParse("5Gi")), "Memory limit should be same as request")
}

func TestGetResourceRequirements_ProvideRequestsCpu_OverDefaultLimits(t *testing.T) {
	request := map[string]string{
		"cpu": "6",
	}

	component := utils.NewDeployComponentBuilder().
		WithResourceRequestsOnly(request).
		BuildComponent()
	requirements := getResourceRequirements(component)

	assert.Equal(t, 0, requirements.Requests.Cpu().Cmp(resource.MustParse("6")), "CPU request should be included")
	assert.Equal(t, 0, requirements.Requests.Memory().Cmp(resource.MustParse("0")), "Missing memory request should be 0")

	assert.Equal(t, 0, requirements.Limits.Cpu().Cmp(resource.MustParse("6")), "CPU limit should be same as request")
	assert.Equal(t, 0, requirements.Limits.Memory().Cmp(resource.MustParse("0")), "Missing memory limit should be 0")
}
