package model_test

import (
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/hash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	applyConfigStep           = &model.DefaultStepImplementation{StepType: pipeline.ApplyConfigStep, SuccessMessage: "config applied"}
	deployConfigStep          = &model.DefaultStepImplementation{StepType: pipeline.DeployConfigStep, SuccessMessage: "config deployed"}
	buildStep                 = &model.DefaultStepImplementation{StepType: pipeline.BuildStep, SuccessMessage: "built"}
	deployStep                = &model.DefaultStepImplementation{StepType: pipeline.DeployStep, SuccessMessage: "deployed"}
	prepareTektonPipelineStep = &model.DefaultStepImplementation{StepType: pipeline.PreparePipelinesStep,
		SuccessMessage: "pipelines prepared"}
	runTektonPipelineStep = &model.DefaultStepImplementation{StepType: pipeline.RunPipelinesStep,
		SuccessMessage: "run pipelines completed"}
)

func Test_DefaultPipeType(t *testing.T) {
	pipelineType, _ := pipeline.GetPipelineFromName("")
	p, _ := model.InitPipeline(pipelineType, &model.PipelineArguments{}, prepareTektonPipelineStep, applyConfigStep, buildStep, runTektonPipelineStep, deployStep)

	assert.Equal(t, v1.BuildDeploy, p.Definition.Type)
	assert.Equal(t, 5, len(p.Steps))
	assert.Equal(t, "pipelines prepared", p.Steps[0].SucceededMsg())
	assert.Equal(t, "config applied", p.Steps[1].SucceededMsg())
	assert.Equal(t, "built", p.Steps[2].SucceededMsg())
	assert.Equal(t, "run pipelines completed", p.Steps[3].SucceededMsg())
	assert.Equal(t, "deployed", p.Steps[4].SucceededMsg())
}

func Test_BuildDeployPipeType(t *testing.T) {
	pipelineType, _ := pipeline.GetPipelineFromName(string(v1.BuildDeploy))
	p, _ := model.InitPipeline(pipelineType, &model.PipelineArguments{}, prepareTektonPipelineStep, applyConfigStep, buildStep, runTektonPipelineStep, deployStep)

	assert.Equal(t, v1.BuildDeploy, p.Definition.Type)
	assert.Equal(t, 5, len(p.Steps))
	assert.Equal(t, "pipelines prepared", p.Steps[0].SucceededMsg())
	assert.Equal(t, "config applied", p.Steps[1].SucceededMsg())
	assert.Equal(t, "built", p.Steps[2].SucceededMsg())
	assert.Equal(t, "run pipelines completed", p.Steps[3].SucceededMsg())
	assert.Equal(t, "deployed", p.Steps[4].SucceededMsg())
}

func Test_BuildAndDefaultNoPushOnlyPipeline(t *testing.T) {
	pipelineType, _ := pipeline.GetPipelineFromName(string(v1.Build))

	p, _ := model.InitPipeline(pipelineType, &model.PipelineArguments{}, prepareTektonPipelineStep, applyConfigStep, buildStep, runTektonPipelineStep, deployStep)
	assert.Equal(t, v1.Build, p.Definition.Type)
	assert.False(t, p.PipelineArguments.PushImage)
	assert.Equal(t, 4, len(p.Steps))
	assert.Equal(t, "pipelines prepared", p.Steps[0].SucceededMsg())
	assert.Equal(t, "config applied", p.Steps[1].SucceededMsg())
	assert.Equal(t, "built", p.Steps[2].SucceededMsg())
	assert.Equal(t, "run pipelines completed", p.Steps[3].SucceededMsg())
}

func Test_ApplyConfigPipeline(t *testing.T) {
	pipelineType, err := pipeline.GetPipelineFromName(string(v1.ApplyConfig))
	require.NoError(t, err, "Failed to get pipeline type. Error %v", err)

	p, err := model.InitPipeline(pipelineType, &model.PipelineArguments{}, prepareTektonPipelineStep, applyConfigStep, deployConfigStep)
	require.NoError(t, err, "Failed to create pipeline. Error %v", err)
	assert.Equal(t, v1.ApplyConfig, p.Definition.Type)
	assert.False(t, p.PipelineArguments.PushImage)
	assert.Equal(t, 3, len(p.Steps))
	assert.Equal(t, "pipelines prepared", p.Steps[0].SucceededMsg())
	assert.Equal(t, "config applied", p.Steps[1].SucceededMsg())
	assert.Equal(t, "config deployed", p.Steps[2].SucceededMsg())
}

func Test_GetImageTagNamesFromArgs(t *testing.T) {
	pipelineType, _ := pipeline.GetPipelineFromName(string(v1.Deploy))
	type scenario struct {
		name                  string
		pipelineArguments     model.PipelineArguments
		expectedToEnvironment string
		expectedImageTagNames map[string]string
	}

	scenarios := []scenario{
		{
			name: "no image tags",
			pipelineArguments: model.PipelineArguments{
				ToEnvironment: "env1",
				ImageTagNames: map[string]string{},
			},
			expectedToEnvironment: "env1",
			expectedImageTagNames: map[string]string{},
		},
		{
			name: "all correct image-tag pairs",
			pipelineArguments: model.PipelineArguments{
				ToEnvironment: "env1",
				ImageTagNames: map[string]string{"component1": "tag1", "component2": "tag22"},
			},
			expectedToEnvironment: "env1",
			expectedImageTagNames: map[string]string{"component1": "tag1", "component2": "tag22"},
		},
	}

	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {

			p, _ := model.InitPipeline(pipelineType, &ts.pipelineArguments, prepareTektonPipelineStep, applyConfigStep, buildStep, runTektonPipelineStep, deployStep)
			assert.Equal(t, v1.Deploy, p.Definition.Type)
			assert.Equal(t, ts.expectedToEnvironment, p.PipelineArguments.ToEnvironment)
			assert.Equal(t, ts.expectedImageTagNames, p.PipelineArguments.ImageTagNames)
		})
	}
}

func Test_BuildOnlyPipeline(t *testing.T) {
	pipelineType, _ := pipeline.GetPipelineFromName(string(v1.Build))

	pipelineArgs := &model.PipelineArguments{
		PushImage: false,
	}

	p, _ := model.InitPipeline(pipelineType, pipelineArgs, prepareTektonPipelineStep, applyConfigStep, buildStep, runTektonPipelineStep, deployStep)
	assert.Equal(t, v1.Build, p.Definition.Type)
	assert.False(t, p.PipelineArguments.PushImage)
	assert.Equal(t, 4, len(p.Steps))
	assert.Equal(t, "pipelines prepared", p.Steps[0].SucceededMsg())
	assert.Equal(t, "config applied", p.Steps[1].SucceededMsg())
	assert.Equal(t, "built", p.Steps[2].SucceededMsg())
	assert.Equal(t, "run pipelines completed", p.Steps[3].SucceededMsg())
}

func Test_BuildAndPushOnlyPipeline(t *testing.T) {
	pipelineType, _ := pipeline.GetPipelineFromName(string(v1.Build))

	pipelineArgs := &model.PipelineArguments{
		PushImage: true,
	}

	p, _ := model.InitPipeline(pipelineType, pipelineArgs, prepareTektonPipelineStep, applyConfigStep, buildStep, runTektonPipelineStep, deployStep)
	assert.Equal(t, v1.Build, p.Definition.Type)
	assert.True(t, p.PipelineArguments.PushImage)
	assert.Equal(t, 4, len(p.Steps))
	assert.Equal(t, "pipelines prepared", p.Steps[0].SucceededMsg())
	assert.Equal(t, "config applied", p.Steps[1].SucceededMsg())
	assert.Equal(t, "built", p.Steps[2].SucceededMsg())
	assert.Equal(t, "run pipelines completed", p.Steps[3].SucceededMsg())
}

func Test_DeployOnlyPipeline(t *testing.T) {
	pipelineType, _ := pipeline.GetPipelineFromName(string(v1.Deploy))

	type scenario struct {
		name                  string
		pipelineArguments     model.PipelineArguments
		expectedToEnvironment string
		expectedImageTagNames map[string]string
	}

	scenarios := []scenario{
		{
			name:                  "only target environment",
			pipelineArguments:     model.PipelineArguments{ToEnvironment: "target"},
			expectedToEnvironment: "target",
		},
		{
			name:                  "target environment with image tags",
			pipelineArguments:     model.PipelineArguments{ToEnvironment: "target", ImageTagNames: map[string]string{"component1": "tag1", "component2": "tag22"}},
			expectedToEnvironment: "target",
			expectedImageTagNames: map[string]string{"component1": "tag1", "component2": "tag22"},
		},
	}

	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			p, _ := model.InitPipeline(pipelineType, &ts.pipelineArguments, prepareTektonPipelineStep, applyConfigStep, runTektonPipelineStep, deployStep)
			assert.Equal(t, v1.Deploy, p.Definition.Type)
			assert.Equal(t, ts.expectedToEnvironment, p.PipelineArguments.ToEnvironment)
			assert.Equal(t, ts.expectedImageTagNames, p.PipelineArguments.ImageTagNames)
			assert.Equal(t, 4, len(p.Steps))
			assert.Equal(t, "pipelines prepared", p.Steps[0].SucceededMsg())
			assert.Equal(t, "config applied", p.Steps[1].SucceededMsg())
			assert.Equal(t, "run pipelines completed", p.Steps[2].SucceededMsg())
			assert.Equal(t, "deployed", p.Steps[3].SucceededMsg())
		})
	}

}

func Test_NonExistingPipelineType(t *testing.T) {
	_, err := pipeline.GetPipelineFromName("non existing pipeline")
	assert.NotNil(t, err)
}

func Test_CompareApplicationWithDeploymentHash(t *testing.T) {
	ra := utils.NewRadixApplicationBuilder().WithAppName("anyname").WithEnvironmentNoBranch("env1").BuildRA()
	testHash, err := hash.CreateRadixApplicationHash(ra)
	require.NoError(t, err)
	otherRa := utils.NewRadixApplicationBuilder().WithAppName("anyname").WithEnvironmentNoBranch("env2").BuildRA()
	otherTestHash, err := hash.CreateRadixApplicationHash(otherRa)
	require.NoError(t, err)

	// Missing ActiveRadixDeployment
	targetEnv := model.TargetEnvironment{}
	isEqual, err := targetEnv.CompareApplicationWithDeploymentHash(ra)
	require.NoError(t, err)
	assert.False(t, isEqual)

	// ActiveRadixDeployment without annotation
	targetEnv = model.TargetEnvironment{ActiveRadixDeployment: utils.NewDeploymentBuilder().BuildRD()}
	isEqual, err = targetEnv.CompareApplicationWithDeploymentHash(ra)
	require.NoError(t, err)
	assert.False(t, isEqual)

	// ActiveRadixDeployment with empty hash in annotation
	targetEnv = model.TargetEnvironment{ActiveRadixDeployment: utils.NewDeploymentBuilder().WithAnnotations(map[string]string{kube.RadixConfigHash: ""}).BuildRD()}
	isEqual, err = targetEnv.CompareApplicationWithDeploymentHash(ra)
	require.NoError(t, err)
	assert.False(t, isEqual)

	// ActiveRadixDeployment with different hash in annotation
	targetEnv = model.TargetEnvironment{ActiveRadixDeployment: utils.NewDeploymentBuilder().WithAnnotations(map[string]string{kube.RadixConfigHash: otherTestHash}).BuildRD()}
	isEqual, err = targetEnv.CompareApplicationWithDeploymentHash(ra)
	require.NoError(t, err)
	assert.False(t, isEqual)

	// ActiveRadixDeployment with matching hash in annotation
	targetEnv = model.TargetEnvironment{ActiveRadixDeployment: utils.NewDeploymentBuilder().WithAnnotations(map[string]string{kube.RadixConfigHash: testHash}).BuildRD()}
	isEqual, err = targetEnv.CompareApplicationWithDeploymentHash(ra)
	require.NoError(t, err)
	assert.True(t, isEqual)
}
