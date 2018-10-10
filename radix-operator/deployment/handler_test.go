package deployment

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/statoil/radix-operator/pkg/apis/utils"
)

func Test_map_rd_to_deploy_name(t *testing.T) {
	file, _ := filepath.Abs("./testdata/radixdeploy.yaml")
	rd, _ := utils.GetRadixDeploy(file)

	component := rd.Spec.Components[0]
	deploy := getDeploymentConfig(rd, component)
	assert.Equal(t, deploy.Name, component.Name)
}

func Test_map_rd_to_deploy_replica(t *testing.T) {
	file, _ := filepath.Abs("./testdata/radixdeploy.yaml")
	rd, _ := utils.GetRadixDeploy(file)

	component := rd.Spec.Components[0]
	deploy := getDeploymentConfig(rd, component)
	replicas := *deploy.Spec.Replicas
	assert.Equal(t, int32(4), replicas)
}

func Test_map_rd_to_deploy_variables(t *testing.T) {
	file, _ := filepath.Abs("./testdata/radixdeploy.yaml")
	rd, _ := utils.GetRadixDeploy(file)

	component := rd.Spec.Components[1]
	deploy := getDeploymentConfig(rd, component)
	assert.Equal(t, "a_variable", deploy.Spec.Template.Spec.Containers[0].Env[0].Name)
	assert.Equal(t, "3001", deploy.Spec.Template.Spec.Containers[0].Env[0].Value)
}

func Test_map_rd_to_deploy_secrets(t *testing.T) {
	file, _ := filepath.Abs("./testdata/radixdeploy.yaml")
	rd, _ := utils.GetRadixDeploy(file)

	component := rd.Spec.Components[2]
	deploy := getDeploymentConfig(rd, component)
	assert.Equal(t, "a_secret", deploy.Spec.Template.Spec.Containers[0].Env[0].Name)
}
