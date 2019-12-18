package model_test

import (
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
)

var (
	copyConfigToMap = &model.DefaultStepImplementation{StepType: pipeline.CopyConfigToMapStep, SuccessMessage: "config copied to map"}
	applyConfigStep = &model.DefaultStepImplementation{StepType: pipeline.ApplyConfigStep, SuccessMessage: "config applied"}
	buildStep       = &model.DefaultStepImplementation{StepType: pipeline.BuildStep, SuccessMessage: "built"}
	scanImageStep   = &model.DefaultStepImplementation{StepType: pipeline.ScanImageStep, SuccessMessage: "image scanned"}
	deployStep      = &model.DefaultStepImplementation{StepType: pipeline.DeployStep, SuccessMessage: "deployed"}
)

func Test_DefaultPipeType(t *testing.T) {
	pipelineType, _ := pipeline.GetPipelineFromName("")
	p, _ := model.InitPipeline(pipelineType, nil, true, model.PipelineArguments{}, copyConfigToMap, applyConfigStep, buildStep, scanImageStep, deployStep)

	assert.Equal(t, v1.BuildDeploy, p.Definition.Type)
	assert.Equal(t, 5, len(p.Steps))
	assert.Equal(t, "config copied to map", p.Steps[0].SucceededMsg())
	assert.Equal(t, "config applied", p.Steps[1].SucceededMsg())
	assert.Equal(t, "built", p.Steps[2].SucceededMsg())
	assert.Equal(t, "image scanned", p.Steps[3].SucceededMsg())
	assert.Equal(t, "deployed", p.Steps[4].SucceededMsg())
}

func Test_BuildDeployPipeType(t *testing.T) {
	pipelineType, _ := pipeline.GetPipelineFromName(string(v1.BuildDeploy))
	p, _ := model.InitPipeline(pipelineType, nil, true, model.PipelineArguments{}, copyConfigToMap, applyConfigStep, buildStep, scanImageStep, deployStep)

	assert.Equal(t, v1.BuildDeploy, p.Definition.Type)
	assert.Equal(t, 5, len(p.Steps))
	assert.Equal(t, "config copied to map", p.Steps[0].SucceededMsg())
	assert.Equal(t, "config applied", p.Steps[1].SucceededMsg())
	assert.Equal(t, "built", p.Steps[2].SucceededMsg())
	assert.Equal(t, "image scanned", p.Steps[3].SucceededMsg())
	assert.Equal(t, "deployed", p.Steps[4].SucceededMsg())
}

func Test_BuildAndDefaultPushOnlyPipeline(t *testing.T) {
	pipelineType, _ := pipeline.GetPipelineFromName(string(v1.Build))

	pipelineArgs := model.GetPipelineArgsFromArguments(make(map[string]string))
	p, _ := model.InitPipeline(pipelineType, nil, true, pipelineArgs, copyConfigToMap, applyConfigStep, buildStep, scanImageStep, deployStep)
	assert.Equal(t, v1.Build, p.Definition.Type)
	assert.True(t, p.PipelineArguments.PushImage)
	assert.Equal(t, 4, len(p.Steps))
	assert.Equal(t, "config copied to map", p.Steps[0].SucceededMsg())
	assert.Equal(t, "config applied", p.Steps[1].SucceededMsg())
	assert.Equal(t, "built", p.Steps[2].SucceededMsg())
	assert.Equal(t, "image scanned", p.Steps[3].SucceededMsg())
}

func Test_BuildOnlyPipeline(t *testing.T) {
	pipelineType, _ := pipeline.GetPipelineFromName(string(v1.Build))

	pipelineArgs := model.PipelineArguments{
		PushImage: false,
	}

	p, _ := model.InitPipeline(pipelineType, nil, true, pipelineArgs, copyConfigToMap, applyConfigStep, buildStep, scanImageStep, deployStep)
	assert.Equal(t, v1.Build, p.Definition.Type)
	assert.False(t, p.PipelineArguments.PushImage)
	assert.Equal(t, 4, len(p.Steps))
	assert.Equal(t, "config copied to map", p.Steps[0].SucceededMsg())
	assert.Equal(t, "config applied", p.Steps[1].SucceededMsg())
	assert.Equal(t, "built", p.Steps[2].SucceededMsg())
	assert.Equal(t, "image scanned", p.Steps[3].SucceededMsg())
}

func Test_BuildAndPushOnlyPipeline(t *testing.T) {
	pipelineType, _ := pipeline.GetPipelineFromName(string(v1.Build))

	pipelineArgs := model.PipelineArguments{
		PushImage: true,
	}

	p, _ := model.InitPipeline(pipelineType, nil, true, pipelineArgs, copyConfigToMap, applyConfigStep, buildStep, scanImageStep, deployStep)
	assert.Equal(t, v1.Build, p.Definition.Type)
	assert.True(t, p.PipelineArguments.PushImage)
	assert.Equal(t, 4, len(p.Steps))
	assert.Equal(t, "config copied to map", p.Steps[0].SucceededMsg())
	assert.Equal(t, "config applied", p.Steps[1].SucceededMsg())
	assert.Equal(t, "built", p.Steps[2].SucceededMsg())
	assert.Equal(t, "image scanned", p.Steps[3].SucceededMsg())
}

func Test_NonExistingPipelineType(t *testing.T) {
	_, err := pipeline.GetPipelineFromName("non existing pipeline")
	assert.NotNil(t, err)
}
