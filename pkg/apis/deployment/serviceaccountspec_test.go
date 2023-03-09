package deployment

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
)

func Test_ServiceAccountSpec(t *testing.T) {

	t.Run("app", func(t *testing.T) {
		t.Parallel()
		rd := utils.NewDeploymentBuilder().
			WithRadixApplication(utils.ARadixApplication()).
			WithAppName("app").
			WithEnvironment("test").
			WithComponents(
				utils.NewDeployComponentBuilder().WithName("app"),
				utils.NewDeployComponentBuilder().
					WithName("app-with-identity").
					WithIdentity(&v1.Identity{Azure: &v1.AzureIdentity{ClientId: "123"}}),
			).
			WithJobComponents(
				utils.NewDeployJobComponentBuilder().WithName("job"),
				utils.NewDeployJobComponentBuilder().
					WithName("job").
					WithIdentity(&v1.Identity{Azure: &v1.AzureIdentity{ClientId: "123"}}),
			).
			BuildRD()

		spec := NewServiceAccountSpec(rd, &rd.Spec.Components[0])
		assert.Equal(t, utils.BoolPtr(false), spec.AutomountServiceAccountToken())
		assert.Equal(t, defaultServiceAccountName, spec.ServiceAccountName())

		spec = NewServiceAccountSpec(rd, &rd.Spec.Components[1])
		assert.Equal(t, utils.BoolPtr(false), spec.AutomountServiceAccountToken())
		assert.Equal(t, utils.GetComponentServiceAccountName(rd.Spec.Components[1].Name), spec.ServiceAccountName())

		spec = NewServiceAccountSpec(rd, newJobSchedulerComponent(&rd.Spec.Jobs[0], rd))
		assert.Equal(t, utils.BoolPtr(true), spec.AutomountServiceAccountToken())
		assert.Equal(t, defaults.RadixJobSchedulerServiceName, spec.ServiceAccountName())

		spec = NewServiceAccountSpec(rd, &rd.Spec.Jobs[0])
		assert.Equal(t, utils.BoolPtr(false), spec.AutomountServiceAccountToken())
		assert.Equal(t, defaultServiceAccountName, spec.ServiceAccountName())

		spec = NewServiceAccountSpec(rd, newJobSchedulerComponent(&rd.Spec.Jobs[1], rd))
		assert.Equal(t, utils.BoolPtr(true), spec.AutomountServiceAccountToken())
		assert.Equal(t, defaults.RadixJobSchedulerServiceName, spec.ServiceAccountName())

		spec = NewServiceAccountSpec(rd, &rd.Spec.Jobs[1])
		assert.Equal(t, utils.BoolPtr(false), spec.AutomountServiceAccountToken())
		assert.Equal(t, utils.GetComponentServiceAccountName(rd.Spec.Jobs[1].Name), spec.ServiceAccountName())
	})

	t.Run("radix api", func(t *testing.T) {
		t.Parallel()
		rd := utils.NewDeploymentBuilder().
			WithRadixApplication(utils.ARadixApplication()).
			WithAppName("radix-api").
			WithEnvironment("test").
			WithComponent(utils.NewDeployComponentBuilder().WithName("app")).
			WithJobComponent(utils.NewDeployJobComponentBuilder().WithName("job")).
			BuildRD()

		spec := NewServiceAccountSpec(rd, &rd.Spec.Components[0])
		assert.Equal(t, utils.BoolPtr(true), spec.AutomountServiceAccountToken())
		assert.Equal(t, defaults.RadixAPIServiceAccountName, spec.ServiceAccountName())

		spec = NewServiceAccountSpec(rd, newJobSchedulerComponent(&rd.Spec.Jobs[0], rd))
		assert.Equal(t, utils.BoolPtr(true), spec.AutomountServiceAccountToken())
		assert.Equal(t, defaults.RadixJobSchedulerServiceName, spec.ServiceAccountName())

		spec = NewServiceAccountSpec(rd, &rd.Spec.Jobs[0])
		assert.Equal(t, utils.BoolPtr(false), spec.AutomountServiceAccountToken())
		assert.Equal(t, defaultServiceAccountName, spec.ServiceAccountName())

	})

	t.Run("radix webhook", func(t *testing.T) {
		t.Parallel()
		rd := utils.NewDeploymentBuilder().
			WithRadixApplication(utils.ARadixApplication()).
			WithAppName("radix-github-webhook").
			WithEnvironment("test").
			WithComponent(utils.NewDeployComponentBuilder().WithName("app")).
			WithJobComponent(utils.NewDeployJobComponentBuilder().WithName("job")).
			BuildRD()

		spec := NewServiceAccountSpec(rd, &rd.Spec.Components[0])
		assert.Equal(t, utils.BoolPtr(true), spec.AutomountServiceAccountToken())
		assert.Equal(t, defaults.RadixGithubWebhookServiceAccountName, spec.ServiceAccountName())

		spec = NewServiceAccountSpec(rd, newJobSchedulerComponent(&rd.Spec.Jobs[0], rd))
		assert.Equal(t, utils.BoolPtr(true), spec.AutomountServiceAccountToken())
		assert.Equal(t, defaults.RadixJobSchedulerServiceName, spec.ServiceAccountName())

		spec = NewServiceAccountSpec(rd, &rd.Spec.Jobs[0])
		assert.Equal(t, utils.BoolPtr(false), spec.AutomountServiceAccountToken())
		assert.Equal(t, defaultServiceAccountName, spec.ServiceAccountName())

	})
}
