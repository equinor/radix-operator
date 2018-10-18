package onpush

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const appTestFilePath = "./testdata/radixconfig.env-branch.yaml"

func Test_get_target_envs_as_map_match_two_envs(t *testing.T) {
	branch := "master"
	ra := createRadixApplication(appTestFilePath)
	targetEnvs := getTargetEnvironmentsAsMap(branch, ra)

	assert.Equal(t, len(targetEnvs), 2)
	assert.Equal(t, targetEnvs["prod"], true)
	assert.Equal(t, targetEnvs["qa"], true)
}

func Test_get_target_envs_as_map_match_one_env(t *testing.T) {
	branch := "development"
	ra := createRadixApplication(appTestFilePath)
	ra.Spec.Environments[0].Build.From = branch
	targetEnvs := getTargetEnvironmentsAsMap(branch, ra)

	assert.Equal(t, len(targetEnvs), 1)
}

func Test_get_target_envs_as_map_no_match(t *testing.T) {
	branch := "development"
	ra := createRadixApplication(appTestFilePath)
	targetEnvs := getTargetEnvironmentsAsMap(branch, ra)

	assert.Equal(t, len(targetEnvs), 0)
	assert.Equal(t, targetEnvs["prod"], false)
	assert.Equal(t, targetEnvs["qa"], false)
}
