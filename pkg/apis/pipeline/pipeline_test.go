package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_StringToPipeline(t *testing.T) {
	_, err := GetPipelineFromName("NA")

	assert.Error(t, err)
}

func Test_StringToPipelineToString(t *testing.T) {
	p, _ := GetPipelineFromName("build-deploy")

	assert.Equal(t, "build-deploy", string(p.Type))

	p, _ = GetPipelineFromName("build")

	assert.Equal(t, "build", string(p.Type))

	p, _ = GetPipelineFromName("apply-config")

	assert.Equal(t, "apply-config", string(p.Type))
}
