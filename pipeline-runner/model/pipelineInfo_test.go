package model_test

import (
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	buildStep  = model.NullStep{SuccessMessage: "built"}
	deployStep = model.NullStep{SuccessMessage: "deployed"}
)

func Test_DefaultPipeType(t *testing.T) {
	p, _ := model.Init("", nil, nil, nil, "", "", "", "", "", "", buildStep, deployStep)

	assert.Equal(t, model.BuildAndDeploy, p.Type)
	assert.Equal(t, 2, len(p.Steps))
	assert.Equal(t, "built", p.Steps[0].SucceededMsg(p))
	assert.Equal(t, "deployed", p.Steps[1].SucceededMsg(p))
}

func Test_BuildDeployPipeType(t *testing.T) {
	p, _ := model.Init(string(model.BuildAndDeploy), nil, nil, nil, "", "", "", "", "", "", buildStep, deployStep)

	assert.Equal(t, model.BuildAndDeploy, p.Type)
	assert.Equal(t, 2, len(p.Steps))
	assert.Equal(t, "built", p.Steps[0].SucceededMsg(p))
	assert.Equal(t, "deployed", p.Steps[1].SucceededMsg(p))
}

func Test_BuildAndDefaultPushOnlyPipeline(t *testing.T) {
	p, _ := model.Init(string(model.Build), nil, nil, nil, "", "", "", "", "", "", buildStep, deployStep)
	assert.Equal(t, model.Build, p.Type)
	assert.True(t, p.PushImage)
	assert.Equal(t, 1, len(p.Steps))
	assert.Equal(t, "built", p.Steps[0].SucceededMsg(p))
}

func Test_BuildOnlyPipeline(t *testing.T) {
	p, _ := model.Init(string(model.Build), nil, nil, nil, "", "", "", "", "", "0", buildStep, deployStep)
	assert.Equal(t, model.Build, p.Type)
	assert.False(t, p.PushImage)
	assert.Equal(t, 1, len(p.Steps))
	assert.Equal(t, "built", p.Steps[0].SucceededMsg(p))
}

func Test_BuildAndPushOnlyPipeline(t *testing.T) {
	p, _ := model.Init(string(model.Build), nil, nil, nil, "", "", "", "", "", "1", buildStep, deployStep)
	assert.Equal(t, model.Build, p.Type)
	assert.True(t, p.PushImage)
	assert.Equal(t, 1, len(p.Steps))
	assert.Equal(t, "built", p.Steps[0].SucceededMsg(p))
}

func Test_NonExistingPipelineType(t *testing.T) {
	_, err := model.Init("non existing pipeline", nil, nil, nil, "", "", "", "", "", "", buildStep, deployStep)
	assert.NotNil(t, err)
}

func Test_AppNameFromRR(t *testing.T) {
	p, _ := model.Init(string(model.Build), &v1.RadixRegistration{
		ObjectMeta: meta_v1.ObjectMeta{Name: "an_app"},
	}, nil, nil, "", "", "", "", "", "", buildStep, deployStep)
	assert.Equal(t, "an_app", p.GetAppName())
}
