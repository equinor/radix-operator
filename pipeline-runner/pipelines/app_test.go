package onpush

import (
	"testing"

	"github.com/statoil/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
)

func TestGetTargetEnvironmentsAsMap_multipleEnvsToOneBranch_ListsBoth(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "master").
		WithEnvironment("prod", "master").
		BuildRA()

	targetEnvs := getTargetEnvironmentsAsMap(branch, ra)

	assert.Equal(t, 2, len(targetEnvs))
	assert.Equal(t, targetEnvs["prod"], true)
	assert.Equal(t, targetEnvs["qa"], true)
}

func TestGetTargetEnvironmentsAsMap_multipleEnvsToOneBranchOtherBranchIsChanged_ListsBothButNoneIsBuilding(t *testing.T) {
	branch := "development"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "master").
		WithEnvironment("prod", "master").
		BuildRA()

	targetEnvs := getTargetEnvironmentsAsMap(branch, ra)

	assert.Equal(t, 2, len(targetEnvs))
	assert.Equal(t, targetEnvs["prod"], false)
	assert.Equal(t, targetEnvs["qa"], false)
}

func TestGetTargetEnvironmentsAsMap_oneEnvToOneBranch_ListsBothButOnlyOneShouldBeBuilt(t *testing.T) {
	branch := "development"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "development").
		WithEnvironment("prod", "master").
		BuildRA()

	targetEnvs := getTargetEnvironmentsAsMap(branch, ra)

	assert.Equal(t, 2, len(targetEnvs))
	assert.Equal(t, targetEnvs["prod"], false)
	assert.Equal(t, targetEnvs["qa"], true)
}

func TestGetTargetEnvironmentsAsMap_twoEnvNoBranch(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironmentNoBranch("qa").
		WithEnvironmentNoBranch("prod").
		BuildRA()

	targetEnvs := getTargetEnvironmentsAsMap(branch, ra)

	assert.Equal(t, 2, len(targetEnvs))
	assert.Equal(t, false, targetEnvs["qa"])
	assert.Equal(t, false, targetEnvs["prod"])
}

func TestGetTargetEnvironmentsAsMap_NoEnv(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		BuildRA()

	targetEnvs := getTargetEnvironmentsAsMap(branch, ra)

	assert.Equal(t, 0, len(targetEnvs))
}

func TestGetTargetEnvironmentsAsMap_promotionScheme_ListsBothButOnlyOneShouldBeBuilt(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "master").
		WithEnvironment("prod", "").
		BuildRA()

	targetEnvs := getTargetEnvironmentsAsMap(branch, ra)

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
