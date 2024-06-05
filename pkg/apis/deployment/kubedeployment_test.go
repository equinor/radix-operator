package deployment

import (
	"context"
	"os"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func teardownReadinessProbe() {
	_ = os.Unsetenv(defaults.OperatorReadinessProbeInitialDelaySeconds)
	_ = os.Unsetenv(defaults.OperatorReadinessProbePeriodSeconds)
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
	jobDeployComponent := &v1.RadixDeployComponent{
		Name: "job1",
	}
	jobAuxDeployment := deploy.createJobAuxDeployment(jobDeployComponent)
	assert.Equal(t, "job1-aux", jobAuxDeployment.GetName())
	resources := jobAuxDeployment.Spec.Template.Spec.Containers[0].Resources
	s := resources.Requests.Cpu().String()
	assert.Equal(t, "1m", s)
	assert.Equal(t, "10M", resources.Requests.Memory().String())
	assert.Equal(t, "0", resources.Limits.Cpu().String())
	assert.Equal(t, "10M", resources.Limits.Memory().String())
}
