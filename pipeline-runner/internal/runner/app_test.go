package runner_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/internal/runner"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"

	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
)

func TestPrepare_NoRegistration_NotValid(t *testing.T) {
	radixclient := radix.NewSimpleClientset() // nolint:staticcheck // SA1019: Ignore linting deprecated fields
	pipelineDefinition, _ := pipeline.GetPipelineFromName(string(v1.BuildDeploy))
	cli := runner.NewRunner(nil, radixclient, nil, nil, pipelineDefinition, "any-app")

	err := cli.PrepareRun(context.Background(), &model.PipelineArguments{})
	assert.Error(t, err)
}
