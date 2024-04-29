package pipeline

import (
	"testing"

	_ "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
)

func Test_RadixComponentSourceBuilder_SinglePropFunc(t *testing.T) {
	// comp := utils.AnApplicationComponent().
	// 	WithName("app").
	// 	WithSourceFolder("/dir").
	// 	WithDockerfileName("dockerfile").
	// 	WithImage("img:latest")

	source := NewComponentImageSourceBuilder().
		WithName("app").
		WithSourceFolder("folder").
		WithDockerfileName("file").
		WithImage("image").
		Build()

	assert.Equal(t, "app", source.Name)
	assert.Equal(t, "folder", source.SourceFolder)
	assert.Equal(t, "file", source.DockerfileName)
	assert.Equal(t, "image", source.Image)
}

func Test_RadixComponentSourceBuilder_RadixComponentSource(t *testing.T) {
	comp := utils.AnApplicationComponent().
		WithName("app").
		WithSourceFolder("folder").
		WithDockerfileName("file").
		WithImage("image").
		BuildComponent()

	source := NewComponentImageSourceBuilder().
		WithSourceFunc(RadixComponentSource(comp)).
		Build()

	assert.Equal(t, "app", source.Name)
	assert.Equal(t, "folder", source.SourceFolder)
	assert.Equal(t, "file", source.DockerfileName)
	assert.Equal(t, "image", source.Image)
}
