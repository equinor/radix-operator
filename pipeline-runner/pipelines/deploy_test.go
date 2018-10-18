package onpush

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const deployTestFilePath = "./testdata/radixconfig.variable.yaml"

func Test_create_radix_deploy(t *testing.T) {
	ra := createRadixApplication(deployTestFilePath)
	targetEnvs := createTargetEnvs()
	deploys, _ := createRadixDeployments(ra, "1", targetEnvs)

	assert.Equal(t, 2, len(deploys))
}

func Test_create_radix_deploy_correct_nr_of_components(t *testing.T) {
	ra := createRadixApplication(deployTestFilePath)
	targetEnvs := createTargetEnvs()
	deploys, _ := createRadixDeployments(ra, "1", targetEnvs)
	devDeploy := deploys[0]

	assert.Equal(t, 2, len(deploys))
	assert.Equal(t, 2, len(devDeploy.Spec.Components))
}

func Test_create_radix_deploy_variable_in_invalid_env(t *testing.T) {
	ra := createRadixApplication(deployTestFilePath)
	targetEnvs := createTargetEnvs()
	deploys, _ := createRadixDeployments(ra, "1", targetEnvs)
	redisComp := deploys[0].Spec.Components[1]
	redisCompProd := deploys[1].Spec.Components[1]

	assert.Equal(t, 2, len(redisComp.EnvironmentVariables))
	assert.Equal(t, 2, len(redisCompProd.EnvironmentVariables))
}

func createTargetEnvs() map[string]bool {
	return map[string]bool{"dev": true, "prod": true}
}
