package deployment

import (
	"os"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func teardownReadinessProbe() {
	_ = os.Unsetenv(defaults.OperatorReadinessProbeInitialDelaySeconds)
	_ = os.Unsetenv(defaults.OperatorReadinessProbePeriodSeconds)
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
	assert.True(t, requirements.Limits.Cpu().IsZero())
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

	assert.True(t, requirements.Limits.Cpu().IsZero())
	assert.Equal(t, 0, requirements.Limits.Memory().Cmp(resource.MustParse("0")), "Missing memory limit should be 0")
}

func TestGetReadinessProbe_MissingDefaultEnvVars(t *testing.T) {
	teardownReadinessProbe()

	probe, err := getReadinessProbeForComponent(&v1.RadixDeployComponent{Ports: []v1.ComponentPort{{Name: "http", Port: int32(80)}}})
	assert.Error(t, err)
	assert.Nil(t, probe)
}

func TestGetReadinessProbe_Custom(t *testing.T) {
	test.SetRequiredEnvironmentVariables()

	probe, err := getReadinessProbeForComponent(&v1.RadixDeployComponent{Ports: []v1.ComponentPort{{Name: "http", Port: int32(5000)}}})
	assert.Nil(t, err)

	assert.Equal(t, int32(5), probe.InitialDelaySeconds)
	assert.Equal(t, int32(10), probe.PeriodSeconds)
	assert.Equal(t, int32(5000), probe.ProbeHandler.TCPSocket.Port.IntVal)

	teardownReadinessProbe()
}

func Test_UpdateResourcesInDeployment(t *testing.T) {
	origRequests := map[string]string{"cpu": "10mi", "memory": "100M"}
	origLimits := map[string]string{"cpu": "100mi", "memory": "1000M"}

	t.Run("set empty requests and limits", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(t, nil, nil)

		expectedRequests := map[string]string{"cpu": "20mi", "memory": "200M"}
		expectedLimits := map[string]string{"cpu": "30mi", "memory": "300M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, expectedLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(&component)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, parseQuantity(expectedRequests["cpu"]), desiredRes.Requests["cpu"])
		assert.Equal(t, parseQuantity(expectedRequests["memory"]), desiredRes.Requests["memory"])
		assert.Equal(t, parseQuantity(expectedLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(expectedLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("set empty requests and update limit", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(t, nil, origLimits)

		expectedRequests := map[string]string{"cpu": "20mi", "memory": "200M"}
		expectedLimits := map[string]string{"cpu": "30mi", "memory": "300M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, expectedLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(&component)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, parseQuantity(expectedRequests["cpu"]), desiredRes.Requests["cpu"])
		assert.Equal(t, parseQuantity(expectedRequests["memory"]), desiredRes.Requests["memory"])
		assert.Equal(t, parseQuantity(expectedLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(expectedLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("update requests and set empty limits", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(t, origRequests, nil)

		expectedRequests := map[string]string{"cpu": "20mi", "memory": "200M"}
		expectedLimits := map[string]string{"cpu": "30mi", "memory": "300M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, expectedLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(&component)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, parseQuantity(expectedRequests["cpu"]), desiredRes.Requests["cpu"])
		assert.Equal(t, parseQuantity(expectedRequests["memory"]), desiredRes.Requests["memory"])
		assert.Equal(t, parseQuantity(expectedLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(expectedLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("update requests and limits", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(t, origRequests, origLimits)

		expectedRequests := map[string]string{"cpu": "20mi", "memory": "200M"}
		expectedLimits := map[string]string{"cpu": "30mi", "memory": "300M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, expectedLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(&component)

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
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(&component)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.NotContains(t, desiredRes.Requests, "cpu")
		assert.Equal(t, parseQuantity(expectedRequests["memory"]), desiredRes.Requests["memory"])
		assert.Equal(t, parseQuantity(origLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(origLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("update requests cpu without memory", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(t, origRequests, origLimits)

		expectedRequests := map[string]string{"cpu": "20mi"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, origLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(&component)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, parseQuantity(expectedRequests["cpu"]), desiredRes.Requests["cpu"])
		assert.NotContains(t, desiredRes.Requests, "memory")
		assert.Equal(t, parseQuantity(origLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(origLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("keep requests and update limits", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(t, origRequests, origLimits)

		expectedLimits := map[string]string{"cpu": "30mi", "memory": "300M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(origRequests, expectedLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(&component)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, parseQuantity(origRequests["cpu"]), desiredRes.Requests["cpu"])
		assert.Equal(t, parseQuantity(origRequests["memory"]), desiredRes.Requests["memory"])
		assert.Equal(t, parseQuantity(expectedLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(expectedLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("update requests and keep limits", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(t, origRequests, origLimits)

		expectedRequests := map[string]string{"cpu": "20mi", "memory": "200M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, origLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(&component)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, parseQuantity(expectedRequests["cpu"]), desiredRes.Requests["cpu"])
		assert.Equal(t, parseQuantity(expectedRequests["memory"]), desiredRes.Requests["memory"])
		assert.Equal(t, parseQuantity(origLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(origLimits["memory"]), desiredRes.Limits["memory"])
	})
	t.Run("remove requests and limits", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(t, origRequests, origLimits)

		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(nil, nil).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(&component)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, 0, len(desiredRes.Requests))
		assert.Equal(t, 0, len(desiredRes.Limits))
	})
	t.Run("update requests and remove limit", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(t, origRequests, origLimits)

		expectedRequests := map[string]string{"cpu": "20mi", "memory": "200M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, nil).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(&component)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, parseQuantity(expectedRequests["cpu"]), desiredRes.Requests["cpu"])
		assert.Equal(t, parseQuantity(expectedRequests["memory"]), desiredRes.Requests["memory"])
		assert.Equal(t, 0, len(desiredRes.Limits))
	})
	t.Run("remove requests and update limit", func(t *testing.T) {
		deployment := applyDeploymentWithSyncWithComponentResources(t, origRequests, origLimits)

		var expectedRequests map[string]string
		expectedLimits := map[string]string{"cpu": "30mi", "memory": "300M"}
		component := utils.NewDeployComponentBuilder().WithName("comp1").WithResource(expectedRequests, expectedLimits).BuildComponent()
		desiredDeployment, _ := deployment.getDesiredCreatedDeploymentConfig(&component)

		desiredRes := desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		assert.Equal(t, 0, len(desiredRes.Requests))
		assert.Equal(t, parseQuantity(expectedLimits["cpu"]), desiredRes.Limits["cpu"])
		assert.Equal(t, parseQuantity(expectedLimits["memory"]), desiredRes.Limits["memory"])
	})
}

func applyDeploymentWithSyncWithComponentResources(t *testing.T, origRequests, origLimits map[string]string) Deployment {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	rd, _ := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient,
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
	jobDeployComponent := &v1.RadixDeployComponent{
		Name: "job1",
	}
	jobAuxDeployment := deploy.createJobAuxDeployment(jobDeployComponent)
	assert.Equal(t, "job1-aux", jobAuxDeployment.GetName())
	resources := jobAuxDeployment.Spec.Template.Spec.Containers[0].Resources
	s := resources.Requests.Cpu().String()
	assert.Equal(t, "50m", s)
	assert.Equal(t, "50M", resources.Requests.Memory().String())
	assert.Equal(t, "50m", resources.Limits.Cpu().String())
	assert.Equal(t, "50M", resources.Limits.Memory().String())
}
