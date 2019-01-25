package application

import (
	"testing"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
)

func getApplication(ra *radixv1.RadixApplication) Application {
	// The other arguments are not relevant for this test
	application, _ := NewApplication(nil, nil, nil, ra)
	return application
}

func TestIsBranchMappedToEnvironment_multipleEnvsToOneBranch_ListsBoth(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "master").
		WithEnvironment("prod", "master").
		BuildRA()

	application := getApplication(ra)
	branchMapped, targetEnvs := application.IsBranchMappedToEnvironment(branch)

	assert.True(t, branchMapped)
	assert.Equal(t, 2, len(targetEnvs))
	assert.Equal(t, targetEnvs["prod"], true)
	assert.Equal(t, targetEnvs["qa"], true)
}

func TestIsBranchMappedToEnvironment_multipleEnvsToOneBranchOtherBranchIsChanged_ListsBothButNoneIsBuilding(t *testing.T) {
	branch := "development"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "master").
		WithEnvironment("prod", "master").
		BuildRA()

	application := getApplication(ra)
	branchMapped, targetEnvs := application.IsBranchMappedToEnvironment(branch)

	assert.False(t, branchMapped)
	assert.Equal(t, 2, len(targetEnvs))
	assert.Equal(t, targetEnvs["prod"], false)
	assert.Equal(t, targetEnvs["qa"], false)
}

func TestIsBranchMappedToEnvironment_oneEnvToOneBranch_ListsBothButOnlyOneShouldBeBuilt(t *testing.T) {
	branch := "development"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "development").
		WithEnvironment("prod", "master").
		BuildRA()

	application := getApplication(ra)
	branchMapped, targetEnvs := application.IsBranchMappedToEnvironment(branch)

	assert.True(t, branchMapped)
	assert.Equal(t, 2, len(targetEnvs))
	assert.Equal(t, targetEnvs["prod"], false)
	assert.Equal(t, targetEnvs["qa"], true)
}

func TestIsBranchMappedToEnvironment_twoEnvNoBranch(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironmentNoBranch("qa").
		WithEnvironmentNoBranch("prod").
		BuildRA()

	application := getApplication(ra)
	branchMapped, targetEnvs := application.IsBranchMappedToEnvironment(branch)

	assert.False(t, branchMapped)
	assert.Equal(t, 2, len(targetEnvs))
	assert.Equal(t, false, targetEnvs["qa"])
	assert.Equal(t, false, targetEnvs["prod"])
}

func TestIsBranchMappedToEnvironment_NoEnv(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		BuildRA()

	application := getApplication(ra)
	branchMapped, targetEnvs := application.IsBranchMappedToEnvironment(branch)

	assert.False(t, branchMapped)
	assert.Equal(t, 0, len(targetEnvs))
}

func TestIsBranchMappedToEnvironment_promotionScheme_ListsBothButOnlyOneShouldBeBuilt(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "master").
		WithEnvironment("prod", "").
		BuildRA()

	application := getApplication(ra)
	branchMapped, targetEnvs := application.IsBranchMappedToEnvironment(branch)

	assert.True(t, branchMapped)
	assert.Equal(t, 2, len(targetEnvs))
	assert.Equal(t, targetEnvs["prod"], false)
	assert.Equal(t, targetEnvs["qa"], true)
}

func TestIsTargetEnvsEmpty_noEntry(t *testing.T) {
	targetEnvs := map[string]bool{}
	assert.Equal(t, true, isTargetEnvsEmpty(targetEnvs))
}

func TestIsTargetEnvsEmpty_twoEntriesWithBranchMapping(t *testing.T) {
	targetEnvs := map[string]bool{
		"qa":   true,
		"prod": true,
	}
	assert.Equal(t, false, isTargetEnvsEmpty(targetEnvs))
}

func TestIsTargetEnvsEmpty_twoEntriesWithNoMapping(t *testing.T) {
	targetEnvs := map[string]bool{
		"qa":   false,
		"prod": false,
	}
	assert.Equal(t, true, isTargetEnvsEmpty(targetEnvs))
}

func TestIsTargetEnvsEmpty_twoEntriesWithOneMapping(t *testing.T) {
	targetEnvs := map[string]bool{
		"qa":   true,
		"prod": false,
	}
	assert.Equal(t, false, isTargetEnvsEmpty(targetEnvs))
}
