package steps

import (
	"context"
	stderrors "errors"
	"fmt"
	"path/filepath"
	"strings"

	commonutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	pipelineDefaults "github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	validate "github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
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
	namespace := operatorutils.GetAppNamespace(cli.GetAppName())
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
	ra, err := CreateRadixApplication(cli.GetRadixclient(), pipelineInfo.PipelineArguments.DNSConfig, configFileContent)
	if err != nil {
		return err
	}

	// Apply RA to cluster
	applicationConfig := application.NewApplicationConfig(cli.GetKubeclient(), cli.GetKubeutil(),
		cli.GetRadixclient(), cli.GetRegistration(), ra,
		pipelineInfo.PipelineArguments.DNSConfig)

	pipelineInfo.SetApplicationConfig(applicationConfig)

	if err := cli.setBuildSecret(pipelineInfo); err != nil {
		return err
	}
	if err := cli.setBuildAndDeployImages(pipelineInfo); err != nil {
		return err
	}

	if pipelineInfo.IsPipelineType(radixv1.BuildDeploy) {
		gitCommitHash, gitTags := cli.getHashAndTags(namespace, pipelineInfo)
		err = validate.GitTagsContainIllegalChars(gitTags)
		if err != nil {
			return err
		}
		pipelineInfo.SetGitAttributes(gitCommitHash, gitTags)
		pipelineInfo.StopPipeline, pipelineInfo.StopPipelineMessage = getPipelineShouldBeStopped(pipelineInfo.PrepareBuildContext)
	}

	if err := cli.validatePipelineInfo(pipelineInfo); err != nil {
		return err
	}

	return applicationConfig.ApplyConfigToApplicationNamespace()
}

func (cli *ApplyConfigStepImplementation) setBuildSecret(pipelineInfo *model.PipelineInfo) error {
	if pipelineInfo.RadixApplication.Spec.Build == nil || len(pipelineInfo.RadixApplication.Spec.Build.Secrets) == 0 {
		return nil
	}

	secret, err := cli.GetKubeclient().CoreV1().Secrets(operatorutils.GetAppNamespace(cli.GetAppName())).Get(context.TODO(), defaults.BuildSecretsName, metav1.GetOptions{})
	if err != nil {
		// For new applications, or when buildsecrets is first added to radixconfig, the secret
		// or role bindings may not be synced yet by radix-operator
		if kubeerrors.IsNotFound(err) || kubeerrors.IsForbidden(err) {
			return nil
		}
		return err
	}
	pipelineInfo.BuildSecret = secret
	return nil
}

func (cli *ApplyConfigStepImplementation) setBuildAndDeployImages(pipelineInfo *model.PipelineInfo) error {
	componentImageSourceMap, err := cli.getEnvironmentComponentImageSource(pipelineInfo)
	if err != nil {
		return err
	}
	setPipelineBuildComponentImages(pipelineInfo, componentImageSourceMap)
	setPipelineDeployEnvironmentComponentImages(pipelineInfo, componentImageSourceMap)
	if pipelineInfo.IsPipelineType(radixv1.BuildDeploy) {
		printEnvironmentComponentImageSources(componentImageSourceMap)
	}
	return nil
}

func (cli *ApplyConfigStepImplementation) validatePipelineInfo(pipelineInfo *model.PipelineInfo) error {
	if pipelineInfo.IsPipelineType(radixv1.Deploy) && len(pipelineInfo.BuildComponentImages) > 0 {
		return ErrDeployOnlyPipelineDoesNotSupportBuild
	}
	if err := validateDeployComponents(pipelineInfo); err != nil {
		return err
	}
	return validateDeployComponentImages(pipelineInfo.DeployEnvironmentComponentImages, pipelineInfo.RadixApplication)
}

func validateDeployComponents(pipelineInfo *model.PipelineInfo) error {
	if len(pipelineInfo.PipelineArguments.ComponentsToDeploy) == 0 {
		return nil
	}
	var errs []error
	componentsMap := getComponentMap(pipelineInfo)
	for _, componentName := range pipelineInfo.PipelineArguments.ComponentsToDeploy {
		componentName = strings.TrimSpace(componentName)
		if len(componentName) == 0 {
			continue
		}
		component, ok := componentsMap[componentName]
		if !ok {
			errs = append(errs, fmt.Errorf("requested component %s does not exist", componentName))
			continue
		}
		for _, env := range pipelineInfo.TargetEnvironments {
			if !component.GetEnabledForEnvironment(env) {
				errs = append(errs, fmt.Errorf("requested component %s is disabled in the environment %s", componentName, env))
			}
		}
	}
	return stderrors.Join(errs...)
}

func getComponentMap(pipelineInfo *model.PipelineInfo) map[string]radixv1.RadixCommonComponent {
	componentsMap := slice.Reduce(pipelineInfo.RadixApplication.Spec.Components, make(map[string]radixv1.RadixCommonComponent), func(acc map[string]radixv1.RadixCommonComponent, component radixv1.RadixComponent) map[string]radixv1.RadixCommonComponent {
		acc[component.GetName()] = &component
		return acc
	})
	componentsMap = slice.Reduce(pipelineInfo.RadixApplication.Spec.Jobs, componentsMap, func(acc map[string]radixv1.RadixCommonComponent, jobComponent radixv1.RadixJobComponent) map[string]radixv1.RadixCommonComponent {
		acc[jobComponent.GetName()] = &jobComponent
		return acc
	})
	return componentsMap
}

func printEnvironmentComponentImageSources(imageSourceMap environmentComponentImageSourceMap) {
	log.Info("Component image source in environments:")
	for envName, componentImageSources := range imageSourceMap {
		log.Infof("  %s:", envName)
		if len(componentImageSources) == 0 {
			log.Info("    - no components to deploy")
			continue
		}
		for _, componentSource := range componentImageSources {
			log.Infof("    - %s from %s", componentSource.ComponentName, getImageSourceDescription(componentSource.ImageSource))
		}
	}
}

func getImageSourceDescription(imageSource containerImageSourceEnum) string {
	switch imageSource {
	case fromDeployment:
		return "active deployment"
	case fromBuild:
		return "build"
	default:
		return "image in radixconfig"
	}
}

type (
	containerImageSourceEnum int

	componentImageSource struct {
		ComponentName string
		ImageSource   containerImageSourceEnum
		Image         string
		Source        radixv1.ComponentSource
	}

	environmentComponentImageSourceMap map[string][]componentImageSource
)

const (
	fromBuild containerImageSourceEnum = iota
	fromImagePath
	fromDeployment
)

func (cli *ApplyConfigStepImplementation) getEnvironmentComponentImageSource(pipelineInfo *model.PipelineInfo) (environmentComponentImageSourceMap, error) {
	ra := pipelineInfo.RadixApplication
	appComponents := getCommonComponents(ra)
	environmentComponentImageSources := make(environmentComponentImageSourceMap)
	for _, envName := range pipelineInfo.TargetEnvironments {
		envNamespace := operatorutils.GetEnvironmentNamespace(ra.GetName(), envName)

		currentRd, err := internal.GetCurrentRadixDeployment(cli.GetKubeutil(), envNamespace)
		if err != nil {
			return nil, err
		}

		mustBuildComponent, err := mustBuildComponentForEnvironment(envName, pipelineInfo.PrepareBuildContext, currentRd, ra, pipelineInfo.BuildSecret)
		if err != nil {
			return nil, err
		}

		enabledComponents := slice.FindAll(appComponents, func(rcc radixv1.RadixCommonComponent) bool { return rcc.GetEnabledForEnvironment(envName) })
		if componentImageSources := getComponentSources(envName, enabledComponents, mustBuildComponent, currentRd); len(componentImageSources) > 0 {
			environmentComponentImageSources[envName] = componentImageSources
		}
	}

	return environmentComponentImageSources, nil
}

func getComponentSources(envName string, components []radixv1.RadixCommonComponent, mustBuildComponent func(comp radixv1.RadixCommonComponent) bool, currentRadixDeployment *radixv1.RadixDeployment) []componentImageSource {
	componentSource := make([]componentImageSource, 0)
	for _, component := range components {
		imageSource := componentImageSource{ComponentName: component.GetName()}
		if image := component.GetImageForEnvironment(envName); len(image) > 0 {
			imageSource.ImageSource = fromImagePath
			imageSource.Image = image
		} else if mustBuildComponent(component) {
			imageSource.ImageSource = fromBuild
			imageSource.Source = component.GetSourceForEnvironment(envName)
		} else {
			imageSource.ImageSource = fromDeployment
			deployComponent := currentRadixDeployment.GetCommonComponentByName(component.GetName())
			if deployComponent == nil {
				continue // component not found in current deployment
			}
			imageSource.Image = deployComponent.GetImage()
		}
		componentSource = append(componentSource, imageSource)
	}
	return componentSource
}

// Get component build information used by build job
func setPipelineBuildComponentImages(pipelineInfo *model.PipelineInfo, componentImageSourceMap environmentComponentImageSourceMap) {
	pipelineInfo.BuildComponentImages = make(pipeline.EnvironmentBuildComponentImages)
	for envName := range componentImageSourceMap {
		buildComponentImages := make(pipeline.BuildComponentImages)
		for imageSourceIndex := 0; imageSourceIndex < len(componentImageSourceMap[envName]); imageSourceIndex++ {
			imageSource := componentImageSourceMap[envName][imageSourceIndex]
			if imageSource.ImageSource != fromBuild {
				continue
			}
			componentName := imageSource.ComponentName
			imageName := fmt.Sprintf("%s-%s", envName, componentName)
			imagePath := operatorutils.GetImagePath(pipelineInfo.PipelineArguments.ContainerRegistry, pipelineInfo.RadixApplication.GetName(), imageName, pipelineInfo.PipelineArguments.ImageTag)
			buildComponentImages[componentName] = pipeline.BuildComponentImage{
				ContainerName: fmt.Sprintf("build-%s-%s", envName, componentName),
				Context:       getContext(imageSource.Source.Folder),
				Dockerfile:    getDockerfileName(imageSource.Source.DockefileName),
				ImageName:     imageName,
				ImagePath:     imagePath,
			}
			componentImageSourceMap[envName][imageSourceIndex].Image = imagePath
		}
		if len(buildComponentImages) > 0 {
			pipelineInfo.BuildComponentImages[envName] = buildComponentImages
		}
	}
}

// Set information about components and image to use for each environment when creating RadixDeployments
func setPipelineDeployEnvironmentComponentImages(pipelineInfo *model.PipelineInfo, environmentImageSourceMap environmentComponentImageSourceMap) {
	pipelineInfo.DeployEnvironmentComponentImages = make(pipeline.DeployEnvironmentComponentImages)
	for envName, imageSources := range environmentImageSourceMap {
		if buildComponentImages, ok := pipelineInfo.BuildComponentImages[envName]; ok {
			pipelineInfo.DeployEnvironmentComponentImages[envName] = slice.Reduce(imageSources, make(pipeline.DeployComponentImages), func(acc pipeline.DeployComponentImages, cis componentImageSource) pipeline.DeployComponentImages {
				deployComponentImage := pipeline.DeployComponentImage{
					ImageTagName: pipelineInfo.PipelineArguments.ImageTagNames[cis.ComponentName],
					Build:        cis.ImageSource == fromBuild,
				}
				if cis.ImageSource == fromBuild {
					deployComponentImage.ImagePath = buildComponentImages[cis.ComponentName].ImagePath
				} else {
					deployComponentImage.ImagePath = cis.Image
				}
				acc[cis.ComponentName] = deployComponentImage
				return acc
			})
		}
	}
}

func isRadixConfigNewOrModifiedSinceDeployment(rd *radixv1.RadixDeployment, ra *radixv1.RadixApplication) (bool, error) {
	if rd == nil {
		return true, nil
	}
	currentRdConfigHash := rd.GetAnnotations()[kube.RadixConfigHash]
	if len(currentRdConfigHash) == 0 {
		return true, nil
	}
	hashEqual, err := compareRadixApplicationHash(currentRdConfigHash, ra)
	if !hashEqual && err == nil {
		log.Infof("RadixApplication updated since last deployment to environment %s", rd.Spec.Environment)
	}
	return !hashEqual, err
}

func isBuildSecretNewOrModifiedSinceDeployment(rd *radixv1.RadixDeployment, buildSecret *corev1.Secret) (bool, error) {
	if rd == nil {
		return true, nil
	}
	targetHash := rd.GetAnnotations()[kube.RadixBuildSecretHash]
	if len(targetHash) == 0 {
		return true, nil
	}
	hashEqual, err := compareBuildSecretHash(targetHash, buildSecret)
	if !hashEqual && err == nil {
		log.Infof("Build secrets updated since last deployment to environment %s", rd.Spec.Environment)
	}
	return !hashEqual, err
}

func mustBuildComponentForEnvironment(environmentName string, prepareBuildContext *model.PrepareBuildContext, currentRd *radixv1.RadixDeployment, ra *radixv1.RadixApplication, buildSecret *corev1.Secret) (func(comp radixv1.RadixCommonComponent) bool, error) {
	alwaysBuild := func(comp radixv1.RadixCommonComponent) bool {
		return true
	}

	if prepareBuildContext == nil || currentRd == nil {
		return alwaysBuild, nil
	}

	if isModified, err := isRadixConfigNewOrModifiedSinceDeployment(currentRd, ra); err != nil {
		return nil, err
	} else if isModified {
		return alwaysBuild, nil
	}

	if isModified, err := isBuildSecretNewOrModifiedSinceDeployment(currentRd, buildSecret); err != nil {
		return nil, err
	} else if isModified {
		return alwaysBuild, nil
	}

	envBuildContext, found := slice.FindFirst(prepareBuildContext.EnvironmentsToBuild, func(etb model.EnvironmentToBuild) bool { return etb.Environment == environmentName })
	if !found {
		return alwaysBuild, nil
	}

	return func(comp radixv1.RadixCommonComponent) bool {
		return slice.Any(envBuildContext.Components, func(s string) bool { return s == comp.GetName() }) ||
			commonutils.IsNil(currentRd.GetCommonComponentByName(comp.GetName()))
	}, nil
}

func getDockerfileName(name string) string {
	if name == "" {
		return "Dockerfile"
	}
	return name
}

func getContext(sourceFolder string) string {
	sourceRoot := filepath.Join("/", sourceFolder)
	return fmt.Sprintf("%s/", filepath.Join(git.Workspace, sourceRoot))
}

func getCommonComponents(ra *radixv1.RadixApplication) []radixv1.RadixCommonComponent {
	commonComponents := slice.Map(ra.Spec.Components, func(c radixv1.RadixComponent) radixv1.RadixCommonComponent { return &c })
	commonComponents = append(commonComponents, slice.Map(ra.Spec.Jobs, func(c radixv1.RadixJobComponent) radixv1.RadixCommonComponent { return &c })...)
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
		log.Infoln("Components with changed source code in environments:")
		for _, environmentToBuild := range prepareBuildContext.EnvironmentsToBuild {
			if len(environmentToBuild.Components) == 0 {
				log.Infof(" - %s: no components or jobs with changed source code", environmentToBuild.Environment)
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
	err = stderrors.Join(commitErr, tagsErr)
	if err != nil {
		log.Errorf("could not retrieve git values from temporary configmap %s, %v", pipelineInfo.GitConfigMapName, err)
		return "", ""
	}
	return gitCommitHash, gitTags
}

// CreateRadixApplication Create RadixApplication from radixconfig.yaml content
func CreateRadixApplication(radixClient radixclient.Interface, dnsConfig *dnsalias.DNSConfig, configFileContent string) (*radixv1.RadixApplication, error) {
	ra := &radixv1.RadixApplication{}

	// Important: Must use sigs.k8s.io/yaml decoder to correctly unmarshal Kubernetes objects.
	// This package supports encoding and decoding of yaml for CRD struct types using the json tag.
	// The gopkg.in/yaml.v3 package requires the yaml tag.
	if err := yaml.Unmarshal([]byte(configFileContent), ra); err != nil {
		return nil, err
	}
	correctRadixApplication(ra)

	// Validate RA
	if validate.RAContainsOldPublic(ra) {
		log.Warnf("component.public is deprecated, please use component.publicPort instead")
	}
	if err := validate.CanRadixApplicationBeInserted(radixClient, ra, dnsConfig); err != nil {
		log.Errorf("Radix config not valid.")
		return nil, err
	}
	return ra, nil
}

func correctRadixApplication(ra *radixv1.RadixApplication) {
	if isAppNameLowercase, err := validate.IsApplicationNameLowercase(ra.Name); !isAppNameLowercase {
		log.Warnf("%s Converting name to lowercase", err.Error())
		ra.Name = strings.ToLower(ra.Name)
	}
	for i := 0; i < len(ra.Spec.Components); i++ {
		ra.Spec.Components[i].Resources = buildResource(&ra.Spec.Components[i])
	}
	for i := 0; i < len(ra.Spec.Jobs); i++ {
		ra.Spec.Jobs[i].Resources = buildResource(&ra.Spec.Jobs[i])
	}
}

func buildResource(component radixv1.RadixCommonComponent) radixv1.ResourceRequirements {
	memoryReqName := corev1.ResourceMemory.String()
	resources := component.GetResources()
	delete(resources.Limits, memoryReqName)

	if requestsMemory, ok := resources.Requests[memoryReqName]; ok {
		if resources.Limits == nil {
			resources.Limits = radixv1.ResourceList{}
		}
		resources.Limits[memoryReqName] = requestsMemory
	}
	return resources
}

func getValueFromConfigMap(key string, configMap *corev1.ConfigMap) (string, error) {
	value, ok := configMap.Data[key]
	if !ok {
		return "", fmt.Errorf("failed to get %s from configMap %s", key, configMap.Name)
	}
	return value, nil
}

func validateDeployComponentImages(deployComponentImages pipeline.DeployEnvironmentComponentImages, ra *radixv1.RadixApplication) error {
	var errs []error

	for envName, components := range deployComponentImages {
		for componentName, imageInfo := range components {
			if strings.HasSuffix(imageInfo.ImagePath, radixv1.DynamicTagNameInEnvironmentConfig) {
				if len(imageInfo.ImageTagName) > 0 {
					continue
				}

				env := ra.GetCommonComponentByName(componentName).GetEnvironmentConfigByName(envName)
				if !commonutils.IsNil(env) && len(env.GetImageTagName()) > 0 {
					continue
				}

				errs = append(errs, errors.WithMessagef(ErrMissingRequiredImageTagName, "component %s in environment %s", componentName, envName))
			}
		}
	}

	return stderrors.Join(errs...)
}
