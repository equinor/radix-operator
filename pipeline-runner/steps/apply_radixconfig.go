package steps

import (
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"strings"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	validate "github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/errors"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
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
	configMap, err := kube.GetConfigMap(cli.GetKubeutil(), namespace, pipelineInfo.RadixConfigMapName)
	if err != nil {
		return err
	}

	ra := &v1.RadixApplication{}
	err = yaml.Unmarshal([]byte(configMap.Data["content"]), ra)

	// Validate RA
	if validate.RAContainsOldPublic(ra) {
		log.Warnf("component.public is deprecated, please use component.publicPort instead")
	}

	isAppNameLowercase, err := validate.IsApplicationNameLowercase(ra.Name)
	if !isAppNameLowercase {
		log.Warnf("%s Converting name to lowercase", err.Error())
		ra.Name = strings.ToLower(ra.Name)
	}

	isRAValid, errs := validate.CanRadixApplicationBeInsertedErrors(cli.GetRadixclient(), ra)
	if !isRAValid {
		log.Errorf("Radix config not valid.")
		return errors.Concat(errs)
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

	return nil
}
