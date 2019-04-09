package model_test

import (
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_DefaultPipeType(t *testing.T) {
	p, _ := model.Init("", nil, nil, nil, "", "", "", "", "", "")
	assert.Equal(t, model.BuildAndDeploy, p.Type)
}

func Test_BuildOnlyPipeline(t *testing.T) {
	p, _ := model.Init(string(model.Build), nil, nil, nil, "", "", "", "", "", "0")
	assert.Equal(t, model.Build, p.Type)
	assert.False(t, p.PushImage)
}

func Test_BuildAndPushOnlyPipeline(t *testing.T) {
	p, _ := model.Init(string(model.Build), nil, nil, nil, "", "", "", "", "", "1")
	assert.Equal(t, model.Build, p.Type)
	assert.True(t, p.PushImage)
}

func Test_BuildAndDefaultPushOnlyPipeline(t *testing.T) {
	p, _ := model.Init(string(model.Build), nil, nil, nil, "", "", "", "", "", "")
	assert.Equal(t, model.Build, p.Type)
	assert.True(t, p.PushImage)
}

func Test_NonExistingPipelineType(t *testing.T) {
	_, err := model.Init("non existing pipeline", nil, nil, nil, "", "", "", "", "", "")
	assert.NotNil(t, err)
}

func Test_AppNameFromRR(t *testing.T) {
	p, _ := model.Init(string(model.Build), &v1.RadixRegistration{
		ObjectMeta: meta_v1.ObjectMeta{Name: "an_app"},
	}, nil, nil, "", "", "", "", "", "")
	assert.Equal(t, "an_app", p.GetAppName())
}
