package models

import (
	"fmt"

	radixhttp "github.com/equinor/radix-common/net/http"
)

// AppNameAndBranchAreRequiredForStartingPipeline Cannot start pipeline when appname and branch are missing
func AppNameAndBranchAreRequiredForStartingPipeline() error {
	return radixhttp.ValidationError("Radix Application Pipeline", "App name and branch are required")
}

// UnmatchedBranchToEnvironment Triggering a pipeline on an un-mapped branch is not allowed
func UnmatchedBranchToEnvironment(branch string) error {
	return radixhttp.ValidationError("Radix Application Pipeline", fmt.Sprintf("Failed to match environment to branch: %s", branch))
}

// EnvironmentNotMappedToBranch Triggering a pipeline on an environment, not matched to a branch
func EnvironmentNotMappedToBranch(envName, branch string) error {
	return radixhttp.ValidationError("Radix Application Pipeline", fmt.Sprintf("Failed to match environment %s to branch: %s", envName, branch))
}
