package steps

import (
	"fmt"

	"github.com/pkg/errors"
)

var (
	ErrDeployOnlyPipelineDoesNotSupportBuild = errors.New("deploy pipeline does not support building components and jobs")
)

func ErrMissingRequiredImageTagName(componentName, envName string) error {
	return fmt.Errorf("component %s requires imageTagName to be set in environmentConfig for environment %s or specified as argument to the pipeline job", componentName, envName)
}
