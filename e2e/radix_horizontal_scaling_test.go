package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHorizontalScalingWithEnvironmentOverride deploys a component with minReplicas and maxReplicas on
// component level, and only maxReplicas in environmentConfig. The environment override must not reset
// minReplicas to a CRD default.
func TestHorizontalScalingWithEnvironmentOverride(t *testing.T) {
	t.Parallel()
	c := getClient(t)
	const (
		appName       = "horizontal-scaling-override-test"
		envName       = "dev"
		componentName = "web"
	)

	activeRD := deployToEnvironmentThroughPipeline(t, c, appName, envName)

	rdComponent := activeRD.GetComponentByName(componentName)
	require.NotNil(t, rdComponent, "RadixDeployment should contain component %s", componentName)
	hs := rdComponent.GetHorizontalScaling()
	require.NotNil(t, hs, "RadixDeployment component %s should have horizontalScaling configured", componentName)
	assert.Equal(t, int32(6), hs.MaxReplicas, "maxReplicas should be overridden by environmentConfig")
	require.NotNil(t, hs.MinReplicas)
	assert.Equal(t, int32(2), *hs.MinReplicas, "minReplicas from component should not be overridden by environmentConfig that does not set minReplicas")
}
