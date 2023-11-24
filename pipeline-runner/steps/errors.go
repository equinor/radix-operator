package steps

import "github.com/pkg/errors"

var (
	ErrDeployOnlyPipelineDoesNotSupportBuild = errors.New("deploy pipeline does not support building components and jobs")
)
