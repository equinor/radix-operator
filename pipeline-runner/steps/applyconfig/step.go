package applyconfig

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	commonutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/runtime"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/hash"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
func (step *ApplyConfigStepImplementation) ImplementationForType() pipeline.StepType {
	return step.stepType
}

// SucceededMsg Override of default step method
func (step *ApplyConfigStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Applied config for application %s", step.GetAppName())
}

// ErrorMsg Override of default step method
func (step *ApplyConfigStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed to apply config for application %s. Error: %v", step.GetAppName(), err)
}

// Run Override of default step method
func (step *ApplyConfigStepImplementation) Run(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
	printPrepareBuildContext(log.Ctx(ctx), pipelineInfo.BuildContext)
	printSubPipelinesToRun(log.Ctx(ctx), pipelineInfo.EnvironmentSubPipelinesToRun)

	if err := step.setBuildSecret(pipelineInfo); err != nil {
		return err
	}

	if slice.Any([]radixv1.RadixPipelineType{radixv1.Build, radixv1.BuildDeploy, radixv1.Deploy}, pipelineInfo.IsPipelineType) {
		if err := step.setBuildAndDeployImages(ctx, pipelineInfo); err != nil {
			return err
		}
	}

	if err := step.validatePipelineInfo(pipelineInfo); err != nil {
		return err
	}

	return step.applyRadixApplication(ctx, pipelineInfo)
}

func (step *ApplyConfigStepImplementation) applyRadixApplication(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
	ra := pipelineInfo.RadixApplication
	namespace := operatorutils.GetAppNamespace(ra.GetName())
	dnsAliasConfig := pipelineInfo.PipelineArguments.DNSConfig

	if err := radixvalidators.CanRadixApplicationBeInserted(ctx, step.GetRadixClient(), ra, dnsAliasConfig); err != nil {
		return err
	}

	existingRA, err := step.GetRadixClient().RadixV1().RadixApplications(namespace).Get(ctx, ra.Name, metav1.GetOptions{})
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			if _, err := step.GetRadixClient().RadixV1().RadixApplications(namespace).Create(ctx, ra, metav1.CreateOptions{}); err != nil {
				return fmt.Errorf("failed to create radix application. %v", err)
			}
			log.Ctx(ctx).Info().Msgf("RadixApplication %s created", ra.Name)
			return nil
		}
		return fmt.Errorf("failed to get radix application. %v", err)
	}

	log.Ctx(ctx).Debug().Msgf("RadixApplication %s exists in namespace %s", ra.Name, namespace)
	if reflect.DeepEqual(ra.Spec, existingRA.Spec) {
		log.Ctx(ctx).Info().Msgf("No changes to RadixApplication %s in namespace %s", ra.Name, namespace)
		return nil
	}

	// Update RA if different
	log.Ctx(ctx).Debug().Msgf("RadixApplication %s in namespace %s has changed, updating now", ra.Name, namespace)
	// For an update, ResourceVersion of the new object must be the same with the old object
	ra.SetResourceVersion(existingRA.GetResourceVersion())
	if _, err = step.GetRadixClient().RadixV1().RadixApplications(namespace).Update(ctx, ra, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to update existing radix application: %w", err)
	}
	log.Ctx(ctx).Info().Msgf("RadixApplication %s updated in namespace %s", ra.Name, namespace)
	return nil
}

func (step *ApplyConfigStepImplementation) setBuildSecret(pipelineInfo *model.PipelineInfo) error {
	if pipelineInfo.RadixApplication.Spec.Build == nil || len(pipelineInfo.RadixApplication.Spec.Build.Secrets) == 0 {
		return nil
	}

	secret, err := step.GetKubeClient().CoreV1().Secrets(operatorutils.GetAppNamespace(step.GetAppName())).Get(context.TODO(), defaults.BuildSecretsName, metav1.GetOptions{})
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

func (step *ApplyConfigStepImplementation) setBuildAndDeployImages(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
	componentImageSourceMap, err := step.getEnvironmentComponentImageSource(ctx, pipelineInfo)
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

func (step *ApplyConfigStepImplementation) validatePipelineInfo(pipelineInfo *model.PipelineInfo) error {

	if err := validateBuildComponents(pipelineInfo); err != nil {
		return err
	}

	if err := validateDeployComponents(pipelineInfo); err != nil {
		return err
	}
	return validateDeployComponentImages(pipelineInfo)
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
	arch := runtime.GetArchitectureFromRuntimeOrDefault(c.Runtime)
	return arch != defaults.DefaultNodeSelectorArchitecture
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
		for _, targetEnv := range pipelineInfo.TargetEnvironments {
			if !component.GetEnabledForEnvironment(targetEnv.Environment) {
				errs = append(errs, fmt.Errorf("requested component %s is disabled in the environment %s", componentName, targetEnv.Environment))
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
			nodeArch := runtime.GetArchitectureFromRuntimeOrDefault(componentSource.Runtime)
			attrs := fmt.Sprintf("arch: %s", nodeArch)
			if nodeType := componentSource.Runtime.GetNodeType(); nodeType != nil {
				attrs = fmt.Sprintf("%s, Node type: %s", attrs, *nodeType)
			}
			log.Ctx(ctx).Info().Msgf("    - %s (%s) from %s", componentSource.ComponentName, attrs, getImageSourceDescription(componentSource.ImageSource))
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

func (step *ApplyConfigStepImplementation) getEnvironmentComponentImageSource(ctx context.Context, pipelineInfo *model.PipelineInfo) (environmentComponentImageSourceMap, error) {
	ra := pipelineInfo.RadixApplication
	appComponents := getCommonComponents(ra)
	environmentComponentImageSources := make(environmentComponentImageSourceMap)
	for _, targetEnv := range pipelineInfo.TargetEnvironments {
		mustBuildComponent, err := mustBuildComponentForEnvironment(ctx, targetEnv, pipelineInfo.BuildContext, pipelineInfo.RadixApplication, pipelineInfo.BuildSecret)
		if err != nil {
			return nil, err
		}
		if componentImageSources, err := step.getComponentSources(appComponents, targetEnv.Environment, targetEnv.ActiveRadixDeployment, mustBuildComponent); err != nil {
			return nil, err
		} else if len(componentImageSources) > 0 {
			environmentComponentImageSources[targetEnv.Environment] = componentImageSources
		}
	}
	return environmentComponentImageSources, nil
}

func (step *ApplyConfigStepImplementation) getComponentSources(appComponents []radixv1.RadixCommonComponent, envName string,
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
			appName := pipelineInfo.RadixApplication.GetName()
			containerRegistry := pipelineInfo.PipelineArguments.ContainerRegistry
			imageTag := pipelineInfo.PipelineArguments.ImageTag
			imagePath := operatorutils.GetImagePath(containerRegistry, appName, imageName, imageTag)
			clusterTypeImagePath := operatorutils.GetImagePath(containerRegistry, appName, imageName, fmt.Sprintf("%s-%s", pipelineInfo.PipelineArguments.Clustertype, imageTag))
			clusterNameImagePath := operatorutils.GetImagePath(containerRegistry, appName, imageName, fmt.Sprintf("%s-%s", pipelineInfo.PipelineArguments.Clustername, imageTag))
			buildComponentImages = append(buildComponentImages, pipeline.BuildComponentImage{
				ComponentName:        componentName,
				EnvName:              envName,
				ContainerName:        containerName,
				Context:              getContext(pipelineInfo.GetGitWorkspace(), imageSource.Source.Folder),
				Dockerfile:           getDockerfileName(imageSource.Source.DockefileName),
				ImageName:            imageName,
				ImagePath:            imagePath,
				ClusterTypeImagePath: clusterTypeImagePath,
				ClusterNameImagePath: clusterNameImagePath,
				Runtime:              imageSource.Runtime,
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
		return fmt.Sprintf("%s-%s", validatedName[:5], strings.ToLower(commonutils.RandStringStrSeed(4, validatedName)))
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

func isBuildSecretNewOrModifiedSinceDeployment(ctx context.Context, rd *radixv1.RadixDeployment, buildSecret *corev1.Secret) (bool, error) {
	if rd == nil {
		return true, nil
	}
	targetHash := rd.GetAnnotations()[kube.RadixBuildSecretHash]
	if len(targetHash) == 0 {
		return true, nil
	}
	hashEqual, err := hash.CompareBuildSecretHash(targetHash, buildSecret)
	if !hashEqual && err == nil {
		log.Ctx(ctx).Info().Msgf("Build secrets updated since last deployment to environment %s", rd.Spec.Environment)
	}
	return !hashEqual, err
}

func mustBuildComponentForEnvironment(ctx context.Context, targetEnvironment model.TargetEnvironment, buildContext *model.BuildContext, ra *radixv1.RadixApplication, buildSecret *corev1.Secret) (func(comp radixv1.RadixCommonComponent) bool, error) {
	alwaysBuild := func(comp radixv1.RadixCommonComponent) bool {
		return true
	}

	if buildContext == nil {
		return alwaysBuild, nil
	}

	if targetEnvironment.ActiveRadixDeployment == nil {
		return alwaysBuild, nil
	}

	if isEqual, err := targetEnvironment.CompareApplicationWithDeploymentHash(ra); err != nil {
		return nil, err
	} else if !isEqual {
		return alwaysBuild, nil
	}

	if isModified, err := isBuildSecretNewOrModifiedSinceDeployment(ctx, targetEnvironment.ActiveRadixDeployment, buildSecret); err != nil {
		return nil, err
	} else if isModified {
		return alwaysBuild, nil
	}

	envBuildContext, found := slice.FindFirst(buildContext.EnvironmentsToBuild, func(etb model.EnvironmentToBuild) bool { return etb.Environment == targetEnvironment.Environment })
	if !found {
		return alwaysBuild, nil
	}

	return func(comp radixv1.RadixCommonComponent) bool {
		return slice.Any(envBuildContext.Components, func(s string) bool { return s == comp.GetName() }) ||
			commonutils.IsNil(targetEnvironment.ActiveRadixDeployment.GetCommonComponentByName(comp.GetName()))
	}, nil
}

func getDockerfileName(name string) string {
	if name == "" {
		return "Dockerfile"
	}
	return name
}

func getContext(workspace, sourceFolder string) string {
	sourceRoot := filepath.Join("/", sourceFolder)
	return fmt.Sprintf("%s/", filepath.Join(workspace, sourceRoot))
}

func getCommonComponents(ra *radixv1.RadixApplication) []radixv1.RadixCommonComponent {
	commonComponents := slice.Map(ra.Spec.Components, func(c radixv1.RadixComponent) radixv1.RadixCommonComponent { return &c })
	commonComponents = append(commonComponents, slice.Map(ra.Spec.Jobs, func(c radixv1.RadixJobComponent) radixv1.RadixCommonComponent { return &c })...)
	return commonComponents
}

func printPrepareBuildContext(logger *zerolog.Logger, buildContext *model.BuildContext) {
	if buildContext == nil {
		return
	}

	if len(buildContext.EnvironmentsToBuild) > 0 {
		logger.Info().Msg("Components with changed source code in environments:")
		for _, environmentToBuild := range buildContext.EnvironmentsToBuild {
			if len(environmentToBuild.Components) == 0 {
				logger.Info().Msgf(" - %s: no components or jobs with changed source code", environmentToBuild.Environment)
			} else {
				logger.Info().Msgf(" - %s: %s", environmentToBuild.Environment, strings.Join(environmentToBuild.Components, ","))
			}
		}
	}
}

func printSubPipelinesToRun(logger *zerolog.Logger, subPipelines []model.EnvironmentSubPipelineToRun) {
	if len(subPipelines) == 0 {
		logger.Info().Msg("No sub-pipelines to run")
	} else {
		logger.Info().Msg("Sub-pipelines to run")
		for _, envSubPipeline := range subPipelines {
			logger.Info().Msgf(" - %s: %s", envSubPipeline.Environment, envSubPipeline.PipelineFile)
		}
	}
}

func validateDeployComponentImages(pipelineInfo *model.PipelineInfo) error {
	var errs []error

	for envName, components := range pipelineInfo.DeployEnvironmentComponentImages {
		for componentName, imageInfo := range components {
			if strings.HasSuffix(imageInfo.ImagePath, radixv1.DynamicTagNameInEnvironmentConfig) {
				if len(imageInfo.ImageTagName) > 0 {
					continue
				}

				component := pipelineInfo.RadixApplication.GetCommonComponentByName(componentName)
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
