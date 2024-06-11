package applyconfig

import "errors"

var (
	ErrDeployOnlyPipelineDoesNotSupportBuild                  = errors.New("deploy pipeline does not support building components and jobs")
	ErrMissingRequiredImageTagName                            = errors.New("missing required imageTagName in a component, an environmentConfig or in a pipeline argument")
	ErrBuildNonDefaultRuntimeArchitectureWithoutBuildKitError = errors.New("build non-AMD64 images without BuildKit enabled is not supported")
)
