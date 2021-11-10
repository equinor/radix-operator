package deployment

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
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
			WithComponent(utils.NewDeployComponentBuilder().WithName("app")).
			WithJobComponent(utils.NewDeployJobComponentBuilder().WithName("job")).
			BuildRD()

		spec := NewServiceAccountSpec(rd, &rd.Spec.Components[0])
		assert.Equal(t, utils.BoolPtr(false), spec.AutomountServiceAccountToken())
		assert.Equal(t, "", spec.ServiceAccountName())

		spec = NewServiceAccountSpec(rd, &rd.Spec.Jobs[0])
		assert.Equal(t, utils.BoolPtr(true), spec.AutomountServiceAccountToken())
		assert.Equal(t, defaults.RadixJobSchedulerServerServiceName, spec.ServiceAccountName())
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

		spec = NewServiceAccountSpec(rd, &rd.Spec.Jobs[0])
		assert.Equal(t, utils.BoolPtr(true), spec.AutomountServiceAccountToken())
		assert.Equal(t, defaults.RadixJobSchedulerServerServiceName, spec.ServiceAccountName())
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

		spec = NewServiceAccountSpec(rd, &rd.Spec.Jobs[0])
		assert.Equal(t, utils.BoolPtr(true), spec.AutomountServiceAccountToken())
		assert.Equal(t, defaults.RadixJobSchedulerServerServiceName, spec.ServiceAccountName())
	})
}
