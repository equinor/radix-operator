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

const (
	multiComponentImageName = "multi-component"
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
	componentImageSources, err := getEnvironmentComponentImageSource(pipelineInfo.TargetEnvironments, pipelineInfo.PrepareBuildContext, pipelineInfo.RadixApplication, pipelineInfo.BuildSecret, cli.GetKubeutil())
	if err != nil {
		return err
	}
	distinctComponentsToBuild := getDistinctComponentsToBuild(componentImageSources, pipelineInfo.RadixApplication)
	multiComponentDockerfile := getMultiComponentDockerfiles(distinctComponentsToBuild)
	pipelineInfo.BuildComponentImages = getBuildComponents(distinctComponentsToBuild, multiComponentDockerfile, pipelineInfo.PipelineArguments.ContainerRegistry, pipelineInfo.PipelineArguments.ImageTag, pipelineInfo.RadixApplication)
	pipelineInfo.DeployEnvironmentComponentImages = getEnvironmentDeployComponents(componentImageSources, pipelineInfo.BuildComponentImages, pipelineInfo.PipelineArguments.ImageTagNames)

	if pipelineInfo.IsPipelineType(radixv1.BuildDeploy) {
		printEnvironmentComponentImageSources(componentImageSources)
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
	if len(pipelineInfo.PipelineArguments.Components) == 0 {
		return nil
	}
	var errs []error
	componentsMap := getComponentMap(pipelineInfo)
	for _, componentName := range pipelineInfo.PipelineArguments.Components {
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
	return nil
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

func printEnvironmentComponentImageSources(imageSources environmentComponentSourceMap) {
	log.Info("Component image source in environments:")
	for envName, envInfo := range imageSources {
		log.Infof("  %s:", envName)
		for _, comp := range envInfo.Components {
			var imageSource string
			switch comp.ImageSource {
			case fromDeployment:
				imageSource = "active deployment"
			case fromBuild:
				imageSource = "build"
			case fromImagePath:
				imageSource = "image in radixconfig"
			}
			log.Infof("    - %s from %s", comp.ComponentName, imageSource)
		}
	}
}

type (
	containerImageSourceEnum int

	componentImageSource struct {
		ComponentName string
		ImageSource   containerImageSourceEnum
	}

	environmentComponentSource struct {
		RadixApplication       *radixv1.RadixApplication
		CurrentRadixDeployment *radixv1.RadixDeployment
		Components             []componentImageSource
	}

	environmentComponentSourceMap map[string]environmentComponentSource

	componentDockerFile struct {
		ComponentName      string
		DockerFileFullPath string
	}
)

const (
	fromBuild containerImageSourceEnum = iota
	fromImagePath
	fromDeployment
)

// Get component image source for each environment
func getEnvironmentComponentImageSource(targetEnvironments []string, prepareBuildContext *model.PrepareBuildContext, ra *radixv1.RadixApplication, buildSecret *corev1.Secret, kubeUtil *kube.Kube) (environmentComponentSourceMap, error) {
	appComponents := getCommonComponents(ra)

	environmentComponents := make(environmentComponentSourceMap)
	for _, envName := range targetEnvironments {
		envNamespace := operatorutils.GetEnvironmentNamespace(ra.GetName(), envName)

		currentRd, err := getCurrentRadixDeployment(kubeUtil, envNamespace)
		if err != nil {
			return nil, err
		}

		mustBuildComponent, err := mustBuildComponentForEnvironment(envName, prepareBuildContext, currentRd, ra, buildSecret)
		if err != nil {
			return nil, err
		}

		enabledComponents := slice.FindAll(appComponents, func(rcc radixv1.RadixCommonComponent) bool { return rcc.GetEnabledForEnvironment(envName) })
		componentSource := getComponentSources(enabledComponents, mustBuildComponent)
		environmentComponents[envName] = environmentComponentSource{
			RadixApplication:       ra,
			CurrentRadixDeployment: currentRd,
			Components:             componentSource,
		}
	}

	return environmentComponents, nil
}

func getComponentSources(components []radixv1.RadixCommonComponent, mustBuildComponent func(comp radixv1.RadixCommonComponent) bool) []componentImageSource {
	componentSource := make([]componentImageSource, 0)

	for _, comp := range components {
		var source containerImageSourceEnum
		switch {
		case len(comp.GetImage()) > 0:
			source = fromImagePath
		case mustBuildComponent(comp):
			source = fromBuild
		default:
			source = fromDeployment
		}
		componentSource = append(componentSource, componentImageSource{ComponentName: comp.GetName(), ImageSource: source})
	}
	return componentSource
}

func getCurrentRadixDeployment(kubeUtil *kube.Kube, namespace string) (*radixv1.RadixDeployment, error) {
	var currentRd *radixv1.RadixDeployment
	// For new applications, or applications with new environments defined in radixconfig, the namespace
	// or rolebinding may not be configured yet by radix-operator.
	// We skip getting active deployment if namespace does not exist or pipeline-runner does not have access

	if _, err := kubeUtil.KubeClient().CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{}); err != nil {
		if !kubeerrors.IsNotFound(err) && !kubeerrors.IsForbidden(err) {
			return nil, err
		}
		log.Infof("namespace for environment does not exist yet: %v", err)
	} else {
		currentRd, err = kubeUtil.GetActiveDeployment(namespace)
		if err != nil {
			return nil, err
		}
	}
	return currentRd, nil
}

// Generate list of distinct components to build across all environments
func getDistinctComponentsToBuild(environmentComponentSource environmentComponentSourceMap, ra *radixv1.RadixApplication) []componentDockerFile {
	var distinctComponentsToBuild []componentDockerFile
	for _, envCompSource := range environmentComponentSource {
		distinctComponentsToBuild = slice.Reduce(envCompSource.Components, distinctComponentsToBuild, func(acc []componentDockerFile, cis componentImageSource) []componentDockerFile {
			if cis.ImageSource == fromBuild {
				if !slice.Any(acc, func(c componentDockerFile) bool { return c.ComponentName == cis.ComponentName }) {
					comp := ra.GetCommonComponentByName(cis.ComponentName)
					acc = append(acc, componentDockerFile{ComponentName: cis.ComponentName, DockerFileFullPath: getDockerfile(comp.GetSourceFolder(), comp.GetDockerfileName())})
				}
			}
			return acc
		})
	}
	return distinctComponentsToBuild
}

// Generate list of dockerfiles used by more than one component/job (multi-component builds)
func getMultiComponentDockerfiles(componentDockerFiles []componentDockerFile) []string {
	return slice.Reduce(componentDockerFiles, []string{}, func(acc []string, cdf componentDockerFile) []string {
		dockerFileCount := len(slice.FindAll(componentDockerFiles, func(c componentDockerFile) bool { return c.DockerFileFullPath == cdf.DockerFileFullPath }))
		if dockerFileCount > 1 && !slice.Any(acc, func(s string) bool { return s == cdf.DockerFileFullPath }) {
			acc = append(acc, cdf.DockerFileFullPath)
		}
		return acc
	})
}

// Get component build information used by build job
func getBuildComponents(componentsDockerFile []componentDockerFile, multiComponentDockerfiles []string, containerRegistry, imageTag string, ra *radixv1.RadixApplication) pipeline.BuildComponentImages {
	return slice.Reduce(componentsDockerFile, make(pipeline.BuildComponentImages), func(acc pipeline.BuildComponentImages, c componentDockerFile) pipeline.BuildComponentImages {
		cc := ra.GetCommonComponentByName(c.ComponentName)
		imageName := c.ComponentName
		if multiComponentIdx := slice.FindIndex(multiComponentDockerfiles, func(s string) bool { return s == c.DockerFileFullPath }); multiComponentIdx >= 0 {
			var suffix string
			if multiComponentIdx > 0 {
				suffix = fmt.Sprintf("-%d", multiComponentIdx)
			}
			imageName = fmt.Sprintf("%s%s", multiComponentImageName, suffix)

		}
		acc[c.ComponentName] = pipeline.BuildComponentImage{
			ContainerName: fmt.Sprintf("build-%s", imageName),
			Context:       getContext(cc.GetSourceFolder()),
			Dockerfile:    getDockerfileName(cc.GetDockerfileName()),
			ImageName:     imageName,
			ImagePath:     operatorutils.GetImagePath(containerRegistry, ra.GetName(), imageName, imageTag),
		}
		return acc
	})
}

// Get information about components and image to use for each environment when creating RadixDeployments
func getEnvironmentDeployComponents(environmentComponentSource environmentComponentSourceMap, buildComponentImages pipeline.BuildComponentImages, imageTagNames map[string]string) pipeline.DeployEnvironmentComponentImages {
	environmentDeployComponentImages := make(pipeline.DeployEnvironmentComponentImages)
	for envName, c := range environmentComponentSource {
		environmentDeployComponentImages[envName] = slice.Reduce(c.Components, make(pipeline.DeployComponentImages), func(acc pipeline.DeployComponentImages, cis componentImageSource) pipeline.DeployComponentImages {
			var imagePath string
			switch cis.ImageSource {
			case fromBuild:
				imagePath = buildComponentImages[cis.ComponentName].ImagePath
			case fromDeployment:
				imagePath = c.CurrentRadixDeployment.GetCommonComponentByName(cis.ComponentName).GetImage()
			case fromImagePath:
				imagePath = c.RadixApplication.GetCommonComponentByName(cis.ComponentName).GetImage()
			}

			acc[cis.ComponentName] = pipeline.DeployComponentImage{
				ImagePath:    imagePath,
				ImageTagName: imageTagNames[cis.ComponentName],
				Build:        cis.ImageSource == fromBuild,
			}
			return acc
		})
	}

	return environmentDeployComponentImages
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

func getDockerfile(sourceFolder, dockerfileName string) string {
	ctx := getContext(sourceFolder)
	dockerfileName = getDockerfileName(dockerfileName)

	return fmt.Sprintf("%s%s", ctx, dockerfileName)
}

func getDockerfileName(name string) string {
	if name == "" {
		name = "Dockerfile"
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
