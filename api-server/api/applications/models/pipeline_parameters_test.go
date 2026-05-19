package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_DontPushToContainerRegistry_WhenPushImage_Is0(t *testing.T) {
	buildParam := PipelineParametersBuild{PushImage: "0"}
	result := buildParam.PushImageToContainerRegistry()

	assert.False(t, result)
}

func Test_DontPushToContainerRegistry_WhenPushImage_IsFalse(t *testing.T) {
	buildParam := PipelineParametersBuild{PushImage: "false"}
	result := buildParam.PushImageToContainerRegistry()

	assert.False(t, result)
}

func Test_PushToContainerRegistry_WhenPushImage_IsNotFalse_Or_IsNot0(t *testing.T) {
	buildParam := PipelineParametersBuild{PushImage: "true"}
	result := buildParam.PushImageToContainerRegistry()

	assert.True(t, result)
}
