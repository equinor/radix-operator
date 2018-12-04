package application

import (
	"strings"

	radixv1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
)

// MagicBranch The branch that radix config lives on
const MagicBranch = "master"

// Application Instance variables
type Application struct {
	config *radixv1.RadixApplication
}

// NewApplication Constructor
func NewApplication(config *radixv1.RadixApplication) Application {
	return Application{config}
}

// IsMagicBranch Checks if given branch is were radix config lives
func IsMagicBranch(branch string) bool {
	return strings.EqualFold(branch, MagicBranch)
}

// IsBranchMappedToEnvironment Checks if given branch has a mapping
func (app Application) IsBranchMappedToEnvironment(branch string) (bool, map[string]bool) {
	targetEnvs := getTargetEnvironmentsAsMap(branch, app.config)
	if isTargetEnvsEmpty(targetEnvs) {
		return false, targetEnvs
	}

	return true, targetEnvs
}

func getTargetEnvironmentsAsMap(branch string, radixApplication *radixv1.RadixApplication) map[string]bool {
	targetEnvs := make(map[string]bool)
	for _, env := range radixApplication.Spec.Environments {
		if branch == env.Build.From {
			// Deploy environment
			targetEnvs[env.Name] = true
		} else {
			// Only create namespace for environment
			targetEnvs[env.Name] = false
		}
	}
	return targetEnvs
}

func isTargetEnvsEmpty(targetEnvs map[string]bool) bool {
	if len(targetEnvs) == 0 {
		return true
	}

	// Check if all values are false
	falseCount := 0
	for _, value := range targetEnvs {
		if value == false {
			falseCount++
		}
	}
	if falseCount == len(targetEnvs) {
		return true
	}

	return false
}
