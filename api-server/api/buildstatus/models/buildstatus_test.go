package models

import (
	"context"
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
)

func Test_PipelineBadgeBuilder(t *testing.T) {
	badgeTemplate := "{{.Operation}}-{{.Status}}"
	badgeBuilder := pipelineBadge{badgeTemplate: badgeTemplate}

	t.Run("failed condition", func(t *testing.T) {
		t.Parallel()
		expected := "-Failing"
		actual, err := badgeBuilder.GetBadge(context.Background(), v1.JobFailed, v1.RadixPipelineType(""))
		assert.Nil(t, err)
		assert.Equal(t, expected, string(actual))
	})
	t.Run("queued condition", func(t *testing.T) {
		t.Parallel()
		expected := "-Pending"
		actual, err := badgeBuilder.GetBadge(context.Background(), v1.JobQueued, v1.RadixPipelineType(""))
		assert.Nil(t, err)
		assert.Equal(t, expected, string(actual))
	})
	t.Run("running condition", func(t *testing.T) {
		t.Parallel()
		expected := "-Running"
		actual, err := badgeBuilder.GetBadge(context.Background(), v1.JobRunning, v1.RadixPipelineType(""))
		assert.Nil(t, err)
		assert.Equal(t, expected, string(actual))
	})
	t.Run("stopped condition", func(t *testing.T) {
		t.Parallel()
		expected := "-Stopped"
		actual, err := badgeBuilder.GetBadge(context.Background(), v1.JobStopped, v1.RadixPipelineType(""))
		assert.Nil(t, err)
		assert.Equal(t, expected, string(actual))
	})
	t.Run("succeeded condition", func(t *testing.T) {
		t.Parallel()
		expected := "-Succeeded"
		actual, err := badgeBuilder.GetBadge(context.Background(), v1.JobSucceeded, v1.RadixPipelineType(""))
		assert.Nil(t, err)
		assert.Equal(t, expected, string(actual))
	})
	t.Run("waiting condition", func(t *testing.T) {
		t.Parallel()
		expected := "-Pending"
		actual, err := badgeBuilder.GetBadge(context.Background(), v1.JobWaiting, v1.RadixPipelineType(""))
		assert.Nil(t, err)
		assert.Equal(t, expected, string(actual))
	})

	t.Run("build-deploy type", func(t *testing.T) {
		t.Parallel()
		expected := "build-deploy-Succeeded"
		actual, err := badgeBuilder.GetBadge(context.Background(), v1.JobSucceeded, v1.BuildDeploy)
		assert.Nil(t, err)
		assert.Equal(t, expected, string(actual))
	})
	t.Run("promote type", func(t *testing.T) {
		t.Parallel()
		expected := "promote-Succeeded"
		actual, err := badgeBuilder.GetBadge(context.Background(), v1.JobSucceeded, v1.Promote)
		assert.Nil(t, err)
		assert.Equal(t, expected, string(actual))
	})
	t.Run("deploy type", func(t *testing.T) {
		t.Parallel()
		expected := "deploy-Succeeded"
		actual, err := badgeBuilder.GetBadge(context.Background(), v1.JobSucceeded, v1.Deploy)
		assert.Nil(t, err)
		assert.Equal(t, expected, string(actual))
	})
	t.Run("unhandled type", func(t *testing.T) {
		t.Parallel()
		expected := "-Succeeded"
		actual, err := badgeBuilder.GetBadge(context.Background(), v1.JobSucceeded, v1.RadixPipelineType("unhandled"))
		assert.Nil(t, err)
		assert.Equal(t, expected, string(actual))
	})

}
