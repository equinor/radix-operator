package onpush

import (
	"path/filepath"
	"testing"

	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"github.com/statoil/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
)

func Test_create_radix_deploy(t *testing.T) {
	ra := createRadixApplication()
	deploys, _ := createRadixDeployments(ra, "1")

	assert.Equal(t, 2, len(deploys))
}

func Test_create_radix_deploy_correct_nr_of_components(t *testing.T) {
	ra := createRadixApplication()
	deploys, _ := createRadixDeployments(ra, "1")
	devDeploy := deploys[0]

	assert.Equal(t, 2, len(deploys))
	assert.Equal(t, 2, len(devDeploy.Spec.Components))
}

func Test_create_radix_deploy_variable_in_invalid_env(t *testing.T) {
	ra := createRadixApplication()
	deploys, _ := createRadixDeployments(ra, "1")
	redisComp := deploys[0].Spec.Components[1]
	redisCompProd := deploys[1].Spec.Components[1]

	assert.Equal(t, 2, len(redisComp.EnvironmentVariables))
	assert.Equal(t, 2, len(redisCompProd.EnvironmentVariables))
}

func createRadixApplication() *v1.RadixApplication {
	fileName, _ := filepath.Abs("./testdata/radixconfig.variable.yaml")
	ra, _ := utils.GetRadixApplication(fileName)
	return ra
}
