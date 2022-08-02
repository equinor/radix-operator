package steps

import (
	"fmt"
	"strings"

	"github.com/equinor/radix-common/utils/errors"
	errorUtils "github.com/equinor/radix-common/utils/errors"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	validate "github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
)

// ApplyConfigStepImplementation Step to apply RA
type ApplyConfigStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
}

// NewApplyConfigStep Constructor
func NewApplyConfigStep() model.Step {
	return &ApplyConfigStepImplementation{
		stepType: pipeline.ApplyConfigStep,
	}
}

// ImplementationForType Override of default step method
func (cli *ApplyConfigStepImplementation) ImplementationForType() pipeline.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *ApplyConfigStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Applied config for application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *ApplyConfigStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed to apply config for application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *ApplyConfigStepImplementation) Run(pipelineInfo *model.PipelineInfo) error {
	// Get radix application from config map
	namespace := utils.GetAppNamespace(cli.GetAppName())
	configMap, err := cli.GetKubeutil().GetConfigMap(namespace, pipelineInfo.RadixConfigMapName)
	if err != nil {
		return err
	}

	configFileContent, ok := configMap.Data["content"]
	if !ok {
		return fmt.Errorf("failed load RadixApplication from ConfigMap")
	}
	ra, err := CreateRadixApplication(cli.GetRadixclient(), configFileContent)
	if err != nil {
		return err
	}

	// Apply RA to cluster
	applicationConfig, err := application.NewApplicationConfig(cli.GetKubeclient(), cli.GetKubeutil(),
		cli.GetRadixclient(), cli.GetRegistration(), ra)
	if err != nil {
		return err
	}

	err = applicationConfig.ApplyConfigToApplicationNamespace()
	if err != nil {
		return err
	}

	// Set back to pipeline
	pipelineInfo.SetApplicationConfig(applicationConfig)

	gitConfigMap, err := cli.GetKubeutil().GetConfigMap(namespace, pipelineInfo.GitConfigMapName)
	if err != nil {
		log.Errorf("could not retrieve git values from temporary configmap %s, %v", pipelineInfo.GitConfigMapName, err)
		return nil
	}
	gitCommitHash, commitErr := getValueFromConfigMap(defaults.RadixGitCommitHashKey, gitConfigMap)
	gitTags, tagsErr := getValueFromConfigMap(defaults.RadixGitTagsKey, gitConfigMap)
	err = errorUtils.Concat([]error{commitErr, tagsErr})
	if err != nil {
		log.Errorf("could not retrieve git values from temporary configmap %s, %v", pipelineInfo.GitConfigMapName, err)
		return nil
	}
	pipelineInfo.SetGitAttributes(gitCommitHash, gitTags)

	return nil
}

//CreateRadixApplication Create RadixApplication from radixconfig.yaml content
func CreateRadixApplication(radixClient radixclient.Interface,
	configFileContent string) (*v1.RadixApplication, error) {
	ra := &v1.RadixApplication{}
	if err := yaml.Unmarshal([]byte(configFileContent), ra); err != nil {
		return nil, err
	}

	// Validate RA
	if validate.RAContainsOldPublic(ra) {
		log.Warnf("component.public is deprecated, please use component.publicPort instead")
	}

	isAppNameLowercase, err := validate.IsApplicationNameLowercase(ra.Name)
	if !isAppNameLowercase {
		log.Warnf("%s Converting name to lowercase", err.Error())
		ra.Name = strings.ToLower(ra.Name)
	}

	isRAValid, errs := validate.CanRadixApplicationBeInsertedErrors(radixClient, ra)
	if !isRAValid {
		log.Errorf("Radix config not valid.")
		return nil, errors.Concat(errs)
	}
	return ra, nil
}

func getValueFromConfigMap(key string, configMap *corev1.ConfigMap) (string, error) {

	value, ok := configMap.Data[key]
	if !ok {
		return "", fmt.Errorf("failed to get %s from configMap %s", key, configMap.Name)
	}
	return value, nil
}
