package clock_test

import (
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/utils/clock"
	"github.com/stretchr/testify/assert"
)

func TestNewFakeClock_ReturnsClockWithProvidedTime(t *testing.T) {
	now := time.Date(2026, time.August, 18, 12, 34, 56, 789, time.UTC)
	clock := clock.NewFakeClock(now)
	assert.Equal(t, now, clock.Now())
}
