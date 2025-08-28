package git_test

import (
	"testing"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/git"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func Test_CloneInitContainers_CustomImages(t *testing.T) {
	type containerInfo struct {
		name  string
		image string
	}
	cfg := git.CloneConfig{GitImage: "anygit:any", BashImage: "anybash:any"}
	containers := git.CloneInitContainersWithSourceCode("anysshurl", "anybranch", "anycommit", "/some-workspace", cfg)
	actual := slice.Map(containers, func(c corev1.Container) containerInfo { return containerInfo{name: c.Name, image: c.Image} })
	expected := []containerInfo{
		{name: git.CloneContainerName, image: cfg.GitImage},
	}
	assert.Equal(t, expected, actual)
}

func Test_CloneInitContainersWithContainerName_CustomImages(t *testing.T) {
	type containerInfo struct {
		name  string
		image string
	}
	cloneName := "anyclonename"
	cfg := git.CloneConfig{GitImage: "anygit:any", BashImage: "anybash:any"}
	containers := git.CloneInitContainersWithContainerName("anysshurl", "anybranch", "anycommit", "/some-workspace", true, false, cloneName, cfg)
	actual := slice.Map(containers, func(c corev1.Container) containerInfo { return containerInfo{name: c.Name, image: c.Image} })
	expected := []containerInfo{
		{name: cloneName, image: cfg.GitImage},
	}
	assert.Equal(t, expected, actual)
}
