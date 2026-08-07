package models

import (
	"encoding/json"
	"testing"
	"time"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_nextCronRun(t *testing.T) {
	t.Run("nil cron schedule is unset", func(t *testing.T) {
		t.Parallel()
		assert.True(t, nextCronRun(nil).IsZero())
	})

	t.Run("empty schedules are unset", func(t *testing.T) {
		t.Parallel()
		assert.True(t, nextCronRun(&radixv1.CronSchedule{Schedules: []string{}}).IsZero())
	})

	t.Run("all invalid schedules are unset", func(t *testing.T) {
		t.Parallel()
		assert.True(t, nextCronRun(&radixv1.CronSchedule{Schedules: []string{"not-a-cron", "still bad"}}).IsZero())
	})

	t.Run("invalid timezone is unset", func(t *testing.T) {
		t.Parallel()
		schedule := &radixv1.CronSchedule{Schedules: []string{"* * * * *"}, TimeZone: "Not/AZone"}
		assert.True(t, nextCronRun(schedule).IsZero())
	})

	t.Run("valid schedule returns next run in UTC", func(t *testing.T) {
		t.Parallel()
		before := time.Now()
		got := nextCronRun(&radixv1.CronSchedule{Schedules: []string{"* * * * *"}})

		require.False(t, got.IsZero())
		assert.Equal(t, time.UTC, got.Location())
		assert.False(t, got.Before(before))
		assert.True(t, got.Before(before.Add(2*time.Minute)))
	})

	t.Run("earliest schedule is chosen", func(t *testing.T) {
		t.Parallel()
		before := time.Now()
		// "0 0 1 1 *" runs once a year, "* * * * *" runs every minute - the latter is always sooner.
		got := nextCronRun(&radixv1.CronSchedule{Schedules: []string{"0 0 1 1 *", "* * * * *"}})

		require.False(t, got.IsZero())
		assert.True(t, got.Before(before.Add(2*time.Minute)))
	})

	t.Run("invalid schedules are skipped when a valid one exists", func(t *testing.T) {
		t.Parallel()
		before := time.Now()
		got := nextCronRun(&radixv1.CronSchedule{Schedules: []string{"garbage", "* * * * *"}})

		require.False(t, got.IsZero())
		assert.True(t, got.Before(before.Add(2*time.Minute)))
	})

	t.Run("timezone is honored", func(t *testing.T) {
		t.Parallel()
		utc := nextCronRun(&radixv1.CronSchedule{Schedules: []string{"0 12 * * *"}, TimeZone: "UTC"})
		ny := nextCronRun(&radixv1.CronSchedule{Schedules: []string{"0 12 * * *"}, TimeZone: "America/New_York"})

		require.False(t, utc.IsZero())
		require.False(t, ny.IsZero())
		// Noon UTC and noon New York are different instants, so their UTC clock hours differ.
		assert.NotEqual(t, utc.UTC().Hour(), ny.UTC().Hour())
	})
}

func Test_ComponentBuilder_NextRun(t *testing.T) {
	rd := &radixv1.RadixDeployment{}
	jobComponent := &radixv1.RadixDeployJobComponent{Name: "job", Image: "job_image"}

	t.Run("next run is set for a cron job", func(t *testing.T) {
		t.Parallel()
		component, err := NewComponentBuilder(rd).
			WithComponent(jobComponent).
			WithNextCronRun(&radixv1.CronSchedule{Schedules: []string{"* * * * *"}}).
			BuildComponent()

		require.NoError(t, err)
		assert.False(t, component.NextRun.IsZero())

		data, err := json.Marshal(component)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"nextRun":`)
	})

	t.Run("next run is omitted when no cron is configured", func(t *testing.T) {
		t.Parallel()
		component, err := NewComponentBuilder(rd).
			WithComponent(jobComponent).
			WithNextCronRun(nil).
			BuildComponent()

		require.NoError(t, err)
		assert.True(t, component.NextRun.IsZero())

		data, err := json.Marshal(component)
		require.NoError(t, err)
		assert.NotContains(t, string(data), "nextRun")
	})

	t.Run("next run is omitted when never set", func(t *testing.T) {
		t.Parallel()
		component, err := NewComponentBuilder(rd).
			WithComponent(jobComponent).
			BuildComponent()

		require.NoError(t, err)
		assert.True(t, component.NextRun.IsZero())

		data, err := json.Marshal(component)
		require.NoError(t, err)
		assert.NotContains(t, string(data), "nextRun")
	})
}
