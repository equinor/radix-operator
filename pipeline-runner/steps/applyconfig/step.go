package applyconfig

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	commonutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	pipelineDefaults "github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pipeline-runner/steps"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	application2 "github.com/equinor/radix-operator/pkg/apis/pipeline/application"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	validate "github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// ApplyConfigStepImplementation Step to apply RA
type ApplyConfigStepImplementation struct {
	stepType steps.StepType
	model.DefaultStepImplementation
}

// NewApplyConfigStep Constructor
func NewApplyConfigStep() model.Step {
	return &ApplyConfigStepImplementation{
		stepType: steps.ApplyConfigStep,
	}
}

// ImplementationForType Override of default step method
func (cli *ApplyConfigStepImplementation) ImplementationForType() steps.StepType {
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
func (cli *ApplyConfigStepImplementation) Run(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
	// Get pipeline info from configmap created by prepare pipeline step
	namespace := operatorutils.GetAppNamespace(cli.GetAppName())
	configMap, err := cli.GetKubeutil().GetConfigMap(ctx, namespace, pipelineInfo.RadixConfigMapName)
	if err != nil {
		return err
	}

	// Read build context info from configmap
	pipelineInfo.PrepareBuildContext, err = getPrepareBuildContext(ctx, configMap)
	if err != nil {
		return err
	}

	// Read radixconfig from configmap
	configFileContent, ok := configMap.Data[pipelineDefaults.PipelineConfigMapContent]
	if !ok {
		return fmt.Errorf("failed load RadixApplication from ConfigMap")
	}
	ra, err := application2.CreateRadixApplication(ctx, cli.GetRadixclient(), pipelineInfo.PipelineArguments.DNSConfig, configFileContent)
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
	if err := cli.setBuildAndDeployImages(ctx, pipelineInfo); err != nil {
		return err
	}

	if pipelineInfo.IsPipelineType(radixv1.BuildDeploy) {
		gitCommitHash, gitTags := cli.getHashAndTags(ctx, namespace, pipelineInfo)
		err = validate.GitTagsContainIllegalChars(gitTags)
		if err != nil {
			return err
		}
		pipelineInfo.SetGitAttributes(gitCommitHash, gitTags)
		pipelineInfo.StopPipeline, pipelineInfo.StopPipelineMessage = getPipelineShouldBeStopped(ctx, pipelineInfo.PrepareBuildContext)
	}

	if err := cli.validatePipelineInfo(pipelineInfo); err != nil {
		return err
	}

	return applicationConfig.ApplyConfigToApplicationNamespace(ctx)
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

func (cli *ApplyConfigStepImplementation) setBuildAndDeployImages(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
	componentImageSourceMap, err := cli.getEnvironmentComponentImageSource(ctx, pipelineInfo)
	if err != nil {
		return err
	}
	setPipelineBuildComponentImages(pipelineInfo, componentImageSourceMap)
	setPipelineDeployEnvironmentComponentImages(pipelineInfo, componentImageSourceMap)
	if pipelineInfo.IsPipelineType(radixv1.BuildDeploy) {
		printEnvironmentComponentImageSources(ctx, componentImageSourceMap)
	}
	return nil
}

func (cli *ApplyConfigStepImplementation) validatePipelineInfo(pipelineInfo *model.PipelineInfo) error {

	if err := validateBuildComponents(pipelineInfo); err != nil {
		return err
	}

	if err := validateDeployComponents(pipelineInfo); err != nil {
		return err
	}
	return validateDeployComponentImages(pipelineInfo.DeployEnvironmentComponentImages, pipelineInfo.RadixApplication)
}

func validateBuildComponents(pipelineInfo *model.PipelineInfo) error {
	if pipelineInfo.IsPipelineType(radixv1.Deploy) && len(pipelineInfo.BuildComponentImages) > 0 {
		return ErrDeployOnlyPipelineDoesNotSupportBuild
	}

	if !pipelineInfo.IsUsingBuildKit() {
		for _, buildComponents := range pipelineInfo.BuildComponentImages {
			if slice.Any(buildComponents, hasNonDefaultRuntimeArchitecture) {
				return ErrBuildNonDefaultRuntimeArchitectureWithoutBuildKitError
			}
		}
	}

	return nil
}

func hasNonDefaultRuntimeArchitecture(c pipeline.BuildComponentImage) bool {
	return operatorutils.GetArchitectureFromRuntime(c.Runtime) != defaults.DefaultNodeSelectorArchitecture
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
	return errors.Join(errs...)
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

func printEnvironmentComponentImageSources(ctx context.Context, imageSourceMap environmentComponentImageSourceMap) {
	log.Ctx(ctx).Info().Msg("Component image source in environments:")
	for envName, componentImageSources := range imageSourceMap {
		log.Ctx(ctx).Info().Msgf("  %s:", envName)
		if len(componentImageSources) == 0 {
			log.Ctx(ctx).Info().Msg("    - no components to deploy")
			continue
		}
		for _, componentSource := range componentImageSources {
			log.Ctx(ctx).Info().Msgf("    - %s (arch: %s) from %s", componentSource.ComponentName, operatorutils.GetArchitectureFromRuntime(componentSource.Runtime), getImageSourceDescription(componentSource.ImageSource))
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
		Runtime       *radixv1.Runtime
	}

	environmentComponentImageSourceMap map[string][]componentImageSource
)

const (
	fromBuild containerImageSourceEnum = iota
	fromImagePath
	fromDeployment
)

func (cli *ApplyConfigStepImplementation) getEnvironmentComponentImageSource(ctx context.Context, pipelineInfo *model.PipelineInfo) (environmentComponentImageSourceMap, error) {
	ra := pipelineInfo.RadixApplication
	appComponents := getCommonComponents(ra)
	environmentComponentImageSources := make(environmentComponentImageSourceMap)
	for _, envName := range pipelineInfo.TargetEnvironments {
		envNamespace := operatorutils.GetEnvironmentNamespace(ra.GetName(), envName)
		activeRadixDeployment, err := internal.GetCurrentRadixDeployment(ctx, cli.GetKubeutil(), envNamespace)
		if err != nil {
			return nil, err
		}

		mustBuildComponent, err := mustBuildComponentForEnvironment(ctx, envName, pipelineInfo.PrepareBuildContext, activeRadixDeployment, pipelineInfo.RadixApplication, pipelineInfo.BuildSecret)
		if err != nil {
			return nil, err
		}
		if componentImageSources, err := cli.getComponentSources(appComponents, envName, activeRadixDeployment, mustBuildComponent); err != nil {
			return nil, err
		} else if len(componentImageSources) > 0 {
			environmentComponentImageSources[envName] = componentImageSources
		}
	}

	return environmentComponentImageSources, nil
}

func (cli *ApplyConfigStepImplementation) getComponentSources(appComponents []radixv1.RadixCommonComponent, envName string,
	activeRadixDeployment *radixv1.RadixDeployment, mustBuildComponent func(comp radixv1.RadixCommonComponent) bool) ([]componentImageSource, error) {

	componentSource := make([]componentImageSource, 0)
	componentsEnabledInEnv := slice.FindAll(appComponents, func(rcc radixv1.RadixCommonComponent) bool { return rcc.GetEnabledForEnvironment(envName) })
	for _, component := range componentsEnabledInEnv {
		imageSource := componentImageSource{ComponentName: component.GetName(), Runtime: internal.GetRuntimeForEnvironment(component, envName)}
		if image := component.GetImageForEnvironment(envName); len(image) > 0 {
			imageSource.ImageSource = fromImagePath
			imageSource.Image = image
		} else {
			currentlyDeployedComponent := getCurrentlyDeployedComponent(activeRadixDeployment, component.GetName())
			if commonutils.IsNil(currentlyDeployedComponent) || mustBuildComponent(component) {
				imageSource.ImageSource = fromBuild
				imageSource.Source = component.GetSourceForEnvironment(envName)
			} else {
				imageSource.ImageSource = fromDeployment
				imageSource.Image = currentlyDeployedComponent.GetImage()
				imageSource.Runtime = currentlyDeployedComponent.GetRuntime()
			}
		}
		componentSource = append(componentSource, imageSource)
	}
	return componentSource, nil
}

func getCurrentlyDeployedComponent(radixDeployment *radixv1.RadixDeployment, componentName string) radixv1.RadixCommonDeployComponent {
	if radixDeployment == nil {
		return nil
	}
	return radixDeployment.GetCommonComponentByName(componentName)
}

// Get component build information used by build job
func setPipelineBuildComponentImages(pipelineInfo *model.PipelineInfo, componentImageSourceMap environmentComponentImageSourceMap) {
	pipelineInfo.BuildComponentImages = make(pipeline.EnvironmentBuildComponentImages)
	for envName := range componentImageSourceMap {
		var buildComponentImages []pipeline.BuildComponentImage
		for imageSourceIndex := 0; imageSourceIndex < len(componentImageSourceMap[envName]); imageSourceIndex++ {
			imageSource := componentImageSourceMap[envName][imageSourceIndex]
			if imageSource.ImageSource != fromBuild {
				continue
			}
			componentName := strings.ToLower(imageSource.ComponentName)
			envNameForName := getLengthLimitedName(envName)
			imageName := fmt.Sprintf("%s-%s", envNameForName, componentName)
			containerName := fmt.Sprintf("build-%s-%s", componentName, envNameForName)
			imagePath := operatorutils.GetImagePath(pipelineInfo.PipelineArguments.ContainerRegistry, pipelineInfo.RadixApplication.GetName(), imageName, pipelineInfo.PipelineArguments.ImageTag)
			buildComponentImages = append(buildComponentImages, pipeline.BuildComponentImage{
				ComponentName: componentName,
				EnvName:       envName,
				ContainerName: containerName,
				Context:       getContext(imageSource.Source.Folder),
				Dockerfile:    getDockerfileName(imageSource.Source.DockefileName),
				ImageName:     imageName,
				ImagePath:     imagePath,
				Runtime:       imageSource.Runtime,
			})
			componentImageSourceMap[envName][imageSourceIndex].Image = imagePath
		}
		if len(buildComponentImages) > 0 {
			pipelineInfo.BuildComponentImages[envName] = buildComponentImages
		}
	}
}

func getLengthLimitedName(name string) string {
	validatedName := strings.ToLower(name)
	if len(validatedName) > 10 {
		return fmt.Sprintf("%s-%s", validatedName[:5], strings.ToLower(commonutils.RandString(4)))
	}
	return validatedName
}

// Set information about components and image to use for each environment when creating RadixDeployments
func setPipelineDeployEnvironmentComponentImages(pipelineInfo *model.PipelineInfo, environmentImageSourceMap environmentComponentImageSourceMap) {
	pipelineInfo.DeployEnvironmentComponentImages = make(pipeline.DeployEnvironmentComponentImages)
	for envName, imageSources := range environmentImageSourceMap {
		pipelineInfo.DeployEnvironmentComponentImages[envName] = slice.Reduce(imageSources, make(pipeline.DeployComponentImages), func(acc pipeline.DeployComponentImages, cis componentImageSource) pipeline.DeployComponentImages {
			deployComponentImage := pipeline.DeployComponentImage{
				ImageTagName: pipelineInfo.PipelineArguments.ImageTagNames[cis.ComponentName],
				Build:        cis.ImageSource == fromBuild,
				Runtime:      cis.Runtime,
			}
			if cis.ImageSource == fromBuild {
				if buildComponentImages, ok := pipelineInfo.BuildComponentImages[envName]; ok {
					if buildComponentImage, ok := slice.FindFirst(buildComponentImages,
						func(componentImage pipeline.BuildComponentImage) bool {
							return componentImage.ComponentName == cis.ComponentName
						}); ok {
						deployComponentImage.ImagePath = buildComponentImage.ImagePath
					} else {
						panic(fmt.Errorf("missing buildComponentImage for the imageSource with ImageSource == fromBuild for environment %s", envName)) // this should not happen, otherwise cis.ImageSource is not consistent
					}
				}
			} else {
				deployComponentImage.ImagePath = cis.Image
			}
			acc[cis.ComponentName] = deployComponentImage
			return acc
		})
	}
}

func isRadixConfigNewOrModifiedSinceDeployment(ctx context.Context, rd *radixv1.RadixDeployment, ra *radixv1.RadixApplication) (bool, error) {
	if rd == nil {
		return true, nil
	}
	currentRdConfigHash := rd.GetAnnotations()[kube.RadixConfigHash]
	if len(currentRdConfigHash) == 0 {
		return true, nil
	}
	hashEqual, err := internal.CompareRadixApplicationHash(currentRdConfigHash, ra)
	if !hashEqual && err == nil {
		log.Ctx(ctx).Info().Msgf("RadixApplication updated since last deployment to environment %s", rd.Spec.Environment)
	}
	return !hashEqual, err
}

func isBuildSecretNewOrModifiedSinceDeployment(ctx context.Context, rd *radixv1.RadixDeployment, buildSecret *corev1.Secret) (bool, error) {
	if rd == nil {
		return true, nil
	}
	targetHash := rd.GetAnnotations()[kube.RadixBuildSecretHash]
	if len(targetHash) == 0 {
		return true, nil
	}
	hashEqual, err := internal.CompareBuildSecretHash(targetHash, buildSecret)
	if !hashEqual && err == nil {
		log.Ctx(ctx).Info().Msgf("Build secrets updated since last deployment to environment %s", rd.Spec.Environment)
	}
	return !hashEqual, err
}

func mustBuildComponentForEnvironment(ctx context.Context, environmentName string, prepareBuildContext *model.PrepareBuildContext, currentRd *radixv1.RadixDeployment, ra *radixv1.RadixApplication, buildSecret *corev1.Secret) (func(comp radixv1.RadixCommonComponent) bool, error) {
	alwaysBuild := func(comp radixv1.RadixCommonComponent) bool {
		return true
	}

	if prepareBuildContext == nil || currentRd == nil {
		return alwaysBuild, nil
	}

	if isModified, err := isRadixConfigNewOrModifiedSinceDeployment(ctx, currentRd, ra); err != nil {
		return nil, err
	} else if isModified {
		return alwaysBuild, nil
	}

	if isModified, err := isBuildSecretNewOrModifiedSinceDeployment(ctx, currentRd, buildSecret); err != nil {
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

func getPrepareBuildContext(ctx context.Context, configMap *corev1.ConfigMap) (*model.PrepareBuildContext, error) {
	prepareBuildContextContent, ok := configMap.Data[pipelineDefaults.PipelineConfigMapBuildContext]
	if !ok {
		log.Ctx(ctx).Debug().Msg("Prepare Build Context does not exist in the ConfigMap")
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
	printPrepareBuildContext(ctx, prepareBuildContext)
	return prepareBuildContext, nil
}

func getPipelineShouldBeStopped(ctx context.Context, prepareBuildContext *model.PrepareBuildContext) (bool, string) {
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
	log.Ctx(ctx).Info().Msg(message)
	return true, message
}

func printPrepareBuildContext(ctx context.Context, prepareBuildContext *model.PrepareBuildContext) {
	if prepareBuildContext.ChangedRadixConfig {
		log.Ctx(ctx).Info().Msg("Radix config file was changed in the repository")
	}
	if len(prepareBuildContext.EnvironmentsToBuild) > 0 {
		log.Ctx(ctx).Info().Msg("Components with changed source code in environments:")
		for _, environmentToBuild := range prepareBuildContext.EnvironmentsToBuild {
			if len(environmentToBuild.Components) == 0 {
				log.Ctx(ctx).Info().Msgf(" - %s: no components or jobs with changed source code", environmentToBuild.Environment)
			} else {
				log.Ctx(ctx).Info().Msgf(" - %s: %s", environmentToBuild.Environment, strings.Join(environmentToBuild.Components, ","))
			}
		}
	}
	if len(prepareBuildContext.EnvironmentSubPipelinesToRun) == 0 {
		log.Ctx(ctx).Info().Msg("No sub-pipelines to run")
	} else {
		log.Ctx(ctx).Info().Msg("Sub-pipelines to run")
		for _, envSubPipeline := range prepareBuildContext.EnvironmentSubPipelinesToRun {
			log.Ctx(ctx).Info().Msgf(" - %s: %s", envSubPipeline.Environment, envSubPipeline.PipelineFile)
		}
	}
}

func (cli *ApplyConfigStepImplementation) getHashAndTags(ctx context.Context, namespace string, pipelineInfo *model.PipelineInfo) (string, string) {
	gitConfigMap, err := cli.GetKubeutil().GetConfigMap(ctx, namespace, pipelineInfo.GitConfigMapName)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msgf("Could not retrieve git values from temporary configmap %s", pipelineInfo.GitConfigMapName)
		return "", ""
	}
	gitCommitHash, commitErr := getValueFromConfigMap(defaults.RadixGitCommitHashKey, gitConfigMap)
	gitTags, tagsErr := getValueFromConfigMap(defaults.RadixGitTagsKey, gitConfigMap)
	err = errors.Join(commitErr, tagsErr)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msgf("could not retrieve git values from temporary configmap %s", pipelineInfo.GitConfigMapName)
		return "", ""
	}
	return gitCommitHash, gitTags
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

				component := ra.GetCommonComponentByName(componentName)
				env := component.GetEnvironmentConfigByName(envName)
				if len(component.GetImageTagName()) > 0 || (!commonutils.IsNil(env) && len(env.GetImageTagName()) > 0) {
					continue
				}

				errs = append(errs, fmt.Errorf("component %s in environment %s: %w", componentName, envName, ErrMissingRequiredImageTagName))
			}
		}
	}

	return errors.Join(errs...)
}
