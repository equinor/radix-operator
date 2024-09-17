package git_test

import (
	"fmt"
	"testing"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

func Test_CloneInitContainers_CustomImages(t *testing.T) {
	type containerInfo struct {
		name  string
		image string
	}
	cfg := git.CloneConfig{NSlookupImage: "anynslookup:any", GitImage: "anygit:any", BashImage: "anybash:any"}
	containers := git.CloneInitContainers("anysshurl", "anybranch", cfg)
	actual := slice.Map(containers, func(c v1.Container) containerInfo { return containerInfo{name: c.Name, image: c.Image} })
	expected := []containerInfo{
		{name: fmt.Sprintf("%snslookup", git.InternalContainerPrefix), image: cfg.NSlookupImage},
		{name: git.CloneContainerName, image: cfg.GitImage},
		{name: fmt.Sprintf("%schmod", git.InternalContainerPrefix), image: cfg.BashImage},
	}
	assert.Equal(t, expected, actual)
}

func Test_CloneInitContainersWithContainerName_CustomImages(t *testing.T) {
	type containerInfo struct {
		name  string
		image string
	}
	cloneName := "anyclonename"
	cfg := git.CloneConfig{NSlookupImage: "anynslookup:any", GitImage: "anygit:any", BashImage: "anybash:any"}
	containers := git.CloneInitContainersWithContainerName("anysshurl", "anybranch", cloneName, cfg)
	actual := slice.Map(containers, func(c v1.Container) containerInfo { return containerInfo{name: c.Name, image: c.Image} })
	expected := []containerInfo{
		{name: fmt.Sprintf("%snslookup", git.InternalContainerPrefix), image: cfg.NSlookupImage},
		{name: cloneName, image: cfg.GitImage},
		{name: fmt.Sprintf("%schmod", git.InternalContainerPrefix), image: cfg.BashImage},
	}
	assert.Equal(t, expected, actual)
}
