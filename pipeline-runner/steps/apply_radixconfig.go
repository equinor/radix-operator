package steps

import (
	"fmt"
	"strings"

	errorUtils "github.com/equinor/radix-common/utils/errors"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	pipelineDefaults "github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	validate "github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	yamlk8s "sigs.k8s.io/yaml"
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
	// Get pipeline info from configmap created by prepare pipeline step
	namespace := utils.GetAppNamespace(cli.GetAppName())
	configMap, err := cli.GetKubeutil().GetConfigMap(namespace, pipelineInfo.RadixConfigMapName)
	if err != nil {
		return err
	}

	// Read build context info from configmap
	pipelineInfo.PrepareBuildContext, err = getPrepareBuildContext(configMap)
	if err != nil {
		return err
	}

	// Read radixconfig from configmap
	configFileContent, ok := configMap.Data[pipelineDefaults.PipelineConfigMapContent]
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

	if err := cli.setBuildAndDeployImages(pipelineInfo); err != nil {
		return err
	}

	if pipelineInfo.PipelineArguments.PipelineType == string(radixv1.BuildDeploy) {
		gitCommitHash, gitTags := cli.getHashAndTags(namespace, pipelineInfo)
		err = validate.GitTagsContainIllegalChars(gitTags)
		if err != nil {
			return err
		}
		pipelineInfo.SetGitAttributes(gitCommitHash, gitTags)
		pipelineInfo.StopPipeline, pipelineInfo.StopPipelineMessage = getPipelineShouldBeStopped(pipelineInfo.PrepareBuildContext)
	}

	return nil
}

type componentName string
type environmentName string

type componentContainersMap map[componentName]containerImageSpec
type environmentComponentsMap map[environmentName]componentContainersMap

type containerImageSource int

const (
	fromBuild containerImageSource = iota
	fromImage
	fromDeployment
)

type containerImageSpec struct {
	ContainerName string
	Context       string
	DockerFile    string
	ImageName     string
	ImagePath     string
	Source        containerImageSource
}

func (cli *ApplyConfigStepImplementation) setBuildAndDeployImages(pipelineInfo *model.PipelineInfo) error {

	appComponents := getCommonComponents(pipelineInfo.RadixApplication)
	env := make(environmentComponentsMap)

	getContainerImage := func(comp radixv1.RadixCommonComponent) containerImageSpec {
		if len(comp.GetImage()) > 0 {
			return containerImageSpec{ImagePath: comp.GetImage(), Source: fromImage}
		}

		return containerImageSpec{}
	}

	for envName, isMapped := range pipelineInfo.TargetEnvironments {
		if !isMapped {
			continue
		}

		currentRd, err := cli.GetKubeutil().GetActiveDeployment(utils.GetEnvironmentNamespace(pipelineInfo.RadixApplication.GetName(), envName))
		if err != nil {
			return err
		}

		enabledComponents := slice.FindAll(appComponents, func(rcc radixv1.RadixCommonComponent) bool { return rcc.GetEnabledForEnvironment(envName) })
		components := make(componentContainersMap)

		for _, comp := range enabledComponents {
			components[componentName(comp.GetName())] = getContainerImage(comp)
		}

		env[environmentName(envName)] = components
	}

	fmt.Println(env)

	// appBuildComponents := getBuildCommonComponents(pipelineInfo.RadixApplication)
	// componentsToBuild := make(map[componentName]radixv1.RadixCommonComponent)

	// for envName, isMapped := range pipelineInfo.TargetEnvironments {
	// 	if !isMapped {
	// 		continue
	// 	}

	// 	currentRd, err := cli.GetKubeutil().GetActiveDeployment(utils.GetEnvironmentNamespace(pipelineInfo.RadixApplication.GetName(), envName))
	// 	if err != nil {
	// 		return err
	// 	}

	// 	envBuildComponents := slice.FindAll(appBuildComponents, func(rcc radixv1.RadixCommonComponent) bool { return rcc.GetEnabledForEnvironment(envName) })
	// 	envMustBuildComponents := filterComponentsForEnvironmentByRequireBuild(envBuildComponents, envName, pipelineInfo.PrepareBuildContext, currentRd)

	// 	for _, comp := range envMustBuildComponents {
	// 		componentsToBuild[componentName(comp.GetName())] = comp
	// 	}
	// }

	return nil
}

func filterComponentsForEnvironmentByRequireBuild(sourceComponents []radixv1.RadixCommonComponent, envName string, prepareBuildCtx *model.PrepareBuildContext, currentRd *radixv1.RadixDeployment) []radixv1.RadixCommonComponent {
	// Build all components if radixconfig.yaml is changed or current RD is nil
	if prepareBuildCtx == nil || prepareBuildCtx.ChangedRadixConfig || currentRd == nil {
		return sourceComponents
	}

	environmentBuildComponentInfo, found := slice.FindFirst(prepareBuildCtx.EnvironmentsToBuild, func(etb model.EnvironmentToBuild) bool { return etb.Environment == envName })
	// Build all components if prepare-pipeline does not have build spec for environment
	if !found {
		return sourceComponents
	}

	var componentsToBuild []radixv1.RadixCommonComponent
	deployComponents := getDeploymentCommonComponents(currentRd)

	for _, sourceComponent := range sourceComponents {
		hasCodeChange := slice.Any(environmentBuildComponentInfo.Components, func(s string) bool { return sourceComponent.GetName() == s })
		existInDeployment := slice.Any(deployComponents, func(rcdc radixv1.RadixCommonDeployComponent) bool { return sourceComponent.GetName() == rcdc.GetName() })

		// Build component if it has code changes or it does not exist in current RadixDeployment
		if hasCodeChange || !existInDeployment {
			componentsToBuild = append(componentsToBuild, sourceComponent)
		}
	}

	return componentsToBuild
}

func getBuildCommonComponents(ra *radixv1.RadixApplication) []radixv1.RadixCommonComponent {
	return slice.FindAll(getCommonComponents(ra), func(rcc radixv1.RadixCommonComponent) bool { return rcc.GetImage() == "" })
}

func getCommonComponents(ra *radixv1.RadixApplication) []radixv1.RadixCommonComponent {
	commonComponents := slice.Map(ra.Spec.Components, func(c radixv1.RadixComponent) radixv1.RadixCommonComponent { return &c })
	commonComponents = append(commonComponents, slice.Map(ra.Spec.Jobs, func(c radixv1.RadixJobComponent) radixv1.RadixCommonComponent { return &c })...)
	return commonComponents
}

func getDeploymentCommonComponents(rd *radixv1.RadixDeployment) []radixv1.RadixCommonDeployComponent {
	commonComponents := slice.Map(rd.Spec.Components, func(c radixv1.RadixDeployComponent) radixv1.RadixCommonDeployComponent { return &c })
	commonComponents = append(commonComponents, slice.Map(rd.Spec.Jobs, func(c radixv1.RadixDeployJobComponent) radixv1.RadixCommonDeployComponent { return &c })...)
	return commonComponents
}

func getPrepareBuildContext(configMap *corev1.ConfigMap) (*model.PrepareBuildContext, error) {
	prepareBuildContextContent, ok := configMap.Data[pipelineDefaults.PipelineConfigMapBuildContext]
	if !ok {
		log.Debug("Prepare Build Context does not exist in the ConfigMap")
		return nil, nil
	}
	prepareBuildContext := &model.PrepareBuildContext{}
	err := yaml.Unmarshal([]byte(prepareBuildContextContent), &prepareBuildContext)
	if err != nil {
		return nil, err
	}
	if prepareBuildContext == nil {
		return nil, nil
	}
	printPrepareBuildContext(prepareBuildContext)
	return prepareBuildContext, nil
}

func getPipelineShouldBeStopped(prepareBuildContext *model.PrepareBuildContext) (bool, string) {
	if prepareBuildContext == nil || prepareBuildContext.ChangedRadixConfig ||
		len(prepareBuildContext.EnvironmentsToBuild) == 0 ||
		len(prepareBuildContext.EnvironmentSubPipelinesToRun) > 0 {
		return false, ""
	}
	for _, environmentToBuild := range prepareBuildContext.EnvironmentsToBuild {
		if len(environmentToBuild.Components) > 0 {
			return false, ""
		}
	}
	message := "No components with changed source code and the Radix config file was not changed. The pipeline will not proceed."
	log.Info(message)
	return true, message
}

func printPrepareBuildContext(prepareBuildContext *model.PrepareBuildContext) {
	if prepareBuildContext.ChangedRadixConfig {
		log.Infoln("Radix config file was changed in the repository")
	}
	if len(prepareBuildContext.EnvironmentsToBuild) > 0 {
		log.Infoln("Components to build in environments:")
		for _, environmentToBuild := range prepareBuildContext.EnvironmentsToBuild {
			if len(environmentToBuild.Components) == 0 {
				log.Infof(" - %s: no components or jobs with changed source", environmentToBuild.Environment)
			} else {
				log.Infof(" - %s: %s", environmentToBuild.Environment, strings.Join(environmentToBuild.Components, ","))
			}
		}
	}
	if len(prepareBuildContext.EnvironmentSubPipelinesToRun) == 0 {
		log.Infoln("No sub-pipelines to run")
	} else {
		log.Infoln("Sub-pipelines to run")
		for _, envSubPipeline := range prepareBuildContext.EnvironmentSubPipelinesToRun {
			log.Infof(" - %s: %s", envSubPipeline.Environment, envSubPipeline.PipelineFile)
		}
	}
}

func (cli *ApplyConfigStepImplementation) getHashAndTags(namespace string, pipelineInfo *model.PipelineInfo) (string, string) {
	gitConfigMap, err := cli.GetKubeutil().GetConfigMap(namespace, pipelineInfo.GitConfigMapName)
	if err != nil {
		log.Errorf("could not retrieve git values from temporary configmap %s, %v", pipelineInfo.GitConfigMapName, err)
		return "", ""
	}
	gitCommitHash, commitErr := getValueFromConfigMap(defaults.RadixGitCommitHashKey, gitConfigMap)
	gitTags, tagsErr := getValueFromConfigMap(defaults.RadixGitTagsKey, gitConfigMap)
	err = errorUtils.Concat([]error{commitErr, tagsErr})
	if err != nil {
		log.Errorf("could not retrieve git values from temporary configmap %s, %v", pipelineInfo.GitConfigMapName, err)
		return "", ""
	}
	return gitCommitHash, gitTags
}

// CreateRadixApplication Create RadixApplication from radixconfig.yaml content
func CreateRadixApplication(radixClient radixclient.Interface,
	configFileContent string) (*radixv1.RadixApplication, error) {
	ra := &radixv1.RadixApplication{}

	// Important: Must use sigs.k8s.io/yaml decoder to correctly unmarshal Kubernetes objects.
	// This package supports encoding and decoding of yaml for CRD struct types using the json tag.
	// The gopkg.in/yaml.v3 package requires the yaml tag.
	if err := yamlk8s.Unmarshal([]byte(configFileContent), ra); err != nil {
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
		return nil, errorUtils.Concat(errs)
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
