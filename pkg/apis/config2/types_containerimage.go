package config2

import (
	"fmt"
)

type ContainerImage struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
}

var _ fmt.Stringer = (*ContainerImage)(nil)

func (ci ContainerImage) String() string {
	if ci.Repository == "" && ci.Tag == "" {
		return ""
	}

	return ci.Repository + ":" + ci.Tag
}
