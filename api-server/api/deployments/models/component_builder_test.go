package models

import (
	"encoding/json"
	"testing"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ComponentBuilder_CronSchedules(t *testing.T) {
	rd := &radixv1.RadixDeployment{}
	jobComponent := &radixv1.RadixDeployJobComponent{Name: "job", Image: "job_image"}

	t.Run("cron schedules set", func(t *testing.T) {
		t.Parallel()
		schedules := []string{"0 0 * * *", "*/5 * * * *"}
		component, err := NewComponentBuilder(rd).
			WithComponent(jobComponent).
			WithCronSchedules(schedules).
			BuildComponent()

		require.NoError(t, err)
		assert.Equal(t, schedules, component.CronSchedules)

		data, err := json.Marshal(component)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"cronSchedules":["0 0 * * *","*/5 * * * *"]`)
	})

	t.Run("empty but set cron schedules render as []", func(t *testing.T) {
		t.Parallel()
		component, err := NewComponentBuilder(rd).
			WithComponent(jobComponent).
			WithCronSchedules([]string{}).
			BuildComponent()

		require.NoError(t, err)
		assert.Equal(t, []string{}, component.CronSchedules)

		data, err := json.Marshal(component)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"cronSchedules":[]`)
	})

	t.Run("nil is omitted from json when not set", func(t *testing.T) {
		t.Parallel()
		component, err := NewComponentBuilder(rd).
			WithComponent(jobComponent).
			BuildComponent()

		require.NoError(t, err)
		assert.Nil(t, component.CronSchedules)

		data, err := json.Marshal(component)
		require.NoError(t, err)
		assert.NotContains(t, string(data), "cronSchedules")
	})
}
