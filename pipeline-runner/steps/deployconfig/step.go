package deployconfig

import (
	"context"
	"fmt"
	"slices"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/internal/watcher"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeployConfigStepImplementation Step to deploy RD into environment
type DeployConfigStepImplementation struct {
	stepType               pipeline.StepType
	namespaceWatcher       watcher.NamespaceWatcher
	radixDeploymentWatcher watcher.RadixDeploymentWatcher
	model.DefaultStepImplementation
}

// NewDeployConfigStep Constructor
func NewDeployConfigStep(namespaceWatcher watcher.NamespaceWatcher, radixDeploymentWatcher watcher.RadixDeploymentWatcher) model.Step {
	return &DeployConfigStepImplementation{
		stepType:               pipeline.DeployConfigStep,
		namespaceWatcher:       namespaceWatcher,
		radixDeploymentWatcher: radixDeploymentWatcher,
	}
}

// ImplementationForType Override of default step method
func (cli *DeployConfigStepImplementation) ImplementationForType() pipeline.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *DeployConfigStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Succeded: deploy application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *DeployConfigStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed to deploy application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *DeployConfigStepImplementation) Run(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
	var updaters []Updater
	if pipelineInfo.PipelineArguments.DeployConfigStep.DeployExternalDNS {
		updaters = append(updaters, &externalDNSDeployer{})
	}
	// if pipelineInfo.PipelineArguments.DeployConfigStep.DeployAppAlias {
	// 	updaters = append(updaters, nil)
	// }

	handler := deployHandler{updaters: updaters, pipelineInfo: *pipelineInfo, kubeutil: cli.GetKubeutil(), rdWatcher: cli.radixDeploymentWatcher}
	return handler.deploy(ctx)
	// return cli.deploy(ctx, pipelineInfo)
}

type Updater interface {
	MustDeployEnvironment(envName string, ra *radixv1.RadixApplication, activeRd *radixv1.RadixDeployment) bool
	UpdateDeployment(target, source *radixv1.RadixDeployment) error
}

type externalDNSDeployer struct{}

func (d *externalDNSDeployer) MustDeployEnvironment(envName string, ra *radixv1.RadixApplication, activeRd *radixv1.RadixDeployment) bool {
	if slices.ContainsFunc(ra.Spec.DNSExternalAlias, func(alias radixv1.ExternalAlias) bool { return alias.Environment == envName }) {
		return true
	}

	if activeRd != nil && slices.ContainsFunc(activeRd.Spec.Components, func(comp radixv1.RadixDeployComponent) bool { return len(comp.GetExternalDNS()) > 0 }) {
		return true
	}

	return false
}

func (d *externalDNSDeployer) UpdateDeployment(target, source *radixv1.RadixDeployment) error {
	for i, targetComp := range target.Spec.Components {
		sourceComp, found := slice.FindFirst(source.Spec.Components, func(c radixv1.RadixDeployComponent) bool { return c.Name == targetComp.Name })
		if !found {
			return fmt.Errorf("component %s not found in active deployment", targetComp.Name)
		}
		target.Spec.Components[i].ExternalDNS = sourceComp.GetExternalDNS()
	}

	return nil
}

type envInfo struct {
	envName  string
	activeRd *radixv1.RadixDeployment
}

type envInfoList []envInfo

type deployHandler struct {
	updaters     []Updater
	kubeutil     *kube.Kube
	pipelineInfo model.PipelineInfo
	rdWatcher    watcher.RadixDeploymentWatcher
}

func (h *deployHandler) deploy(ctx context.Context) error {
	envsToDeploy, err := h.getEnvironmentsToDeploy(ctx)
	if err != nil {
		return fmt.Errorf("failed to get list of environments to deploy: %w", err)
	}

	if err := h.validateEnvironmentsToDeploy(envsToDeploy); err != nil {
		return fmt.Errorf("failed to validate environments: %w", err)
	}

	return h.deployEnvironments(ctx, envsToDeploy)
}

func (h *deployHandler) validateEnvironmentsToDeploy(envsToDeploy envInfoList) error {
	for _, envInfo := range envsToDeploy {
		if envInfo.activeRd == nil {
			return fmt.Errorf("cannot deploy to environment %s because it does not have an active Radix deployment", envInfo.envName)
		}
	}
	return nil
}

func (h *deployHandler) getEnvironmentsToDeploy(ctx context.Context) (envInfoList, error) {
	allEnvs := envInfoList{}
	for _, env := range h.pipelineInfo.RadixApplication.Spec.Environments {
		envNs := utils.GetEnvironmentNamespace(h.pipelineInfo.RadixApplication.GetName(), env.Name)
		rd, err := internal.GetActiveRadixDeployment(ctx, h.kubeutil, envNs)
		if err != nil {
			return nil, fmt.Errorf("failed to get active Radix deployment for environment %s: %w", env.Name, err)
		}
		allEnvs = append(allEnvs, envInfo{envName: env.Name, activeRd: rd})
	}

	deployEnvs := envInfoList{}
	for _, envInfo := range allEnvs {
		if slices.ContainsFunc(h.updaters, func(deployer Updater) bool {
			return deployer.MustDeployEnvironment(envInfo.envName, h.pipelineInfo.RadixApplication, envInfo.activeRd)
		}) {
			deployEnvs = append(deployEnvs, envInfo)
		}
	}

	return deployEnvs, nil
}

func (h *deployHandler) deployEnvironments(ctx context.Context, envs envInfoList) error {
	rdList, err := h.buildDeployments(ctx, envs)
	if err != nil {
		return fmt.Errorf("failed to build Radix deployments: %w", err)
	}

	// Compare new deployments with current active deployments on only deploy changed ones

	// create deployments
	for _, rd := range rdList {
		log.Ctx(ctx).Info().Msgf("Apply Radix deployment %s to environment %s", rd.GetName(), rd.Spec.Environment)
		_, err := h.kubeutil.RadixClient().RadixV1().RadixDeployments(rd.GetNamespace()).Create(ctx, rd, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to apply Radix deployment %s to environment %s: %w", rd.GetName(), rd.Spec.Environment, err)
		}
	}

	// wait for activation
	var g errgroup.Group
	for _, rd := range rdList {
		g.Go(func() error {
			if err := h.rdWatcher.WaitForActive(ctx, rd.GetNamespace(), rd.GetName()); err != nil {
				return fmt.Errorf("failed to wait for activation of Radix deployment %s in environment %s: %w", rd.GetName(), rd.Spec.Environment, err)
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("failed to wait for activation of Radix deployments: %w", err)
	}

	return nil
}

func (h *deployHandler) buildDeployments(ctx context.Context, envs envInfoList) ([]*radixv1.RadixDeployment, error) {
	var rdList []*radixv1.RadixDeployment

	for _, envInfo := range envs {
		rd, err := h.buildDeployment(ctx, envInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to build Radix deployment for environment %s: %w", envInfo.envName, err)
		}

		// Validate that the same components and jobs exist in current acxtive and new deployment

		// Compare components (and jobs) in current active and new RD. Add to list if different
		rdList = append(rdList, rd)
	}

	return rdList, nil
}

func (h *deployHandler) buildDeployment(ctx context.Context, envInfo envInfo) (*radixv1.RadixDeployment, error) {
	// TODO: should we use the new RA hash or hash from current active RD?
	// Both can cause inconsitency issues since we technically do not apply everything from RA to the RDs
	// radixApplicationHash, err := internal.CreateRadixApplicationHash(h.pipelineInfo.RadixApplication)
	// if err != nil {
	// 	return nil, err
	// }

	sourceRd, err := internal.ConstructForTargetEnvironment(
		ctx,
		h.pipelineInfo.RadixApplication,
		envInfo.activeRd,
		h.pipelineInfo.PipelineArguments.JobName,
		h.pipelineInfo.PipelineArguments.ImageTag,
		envInfo.activeRd.Annotations[kube.RadixBranchAnnotation],
		envInfo.activeRd.Annotations[kube.RadixCommitAnnotation],
		envInfo.activeRd.Annotations[kube.RadixGitTagsAnnotation],
		nil,
		envInfo.envName,
		envInfo.activeRd.Annotations[kube.RadixConfigHash],
		envInfo.activeRd.Annotations[kube.RadixBuildSecretHash],
		h.pipelineInfo.PrepareBuildContext,
		h.pipelineInfo.PipelineArguments.ComponentsToDeploy,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to construct Radix deployment: %w", err)
	}

	targetRd := envInfo.activeRd.DeepCopy()
	targetRd.ObjectMeta = *sourceRd.ObjectMeta.DeepCopy()

	for _, updater := range h.updaters {
		if err := updater.UpdateDeployment(targetRd, sourceRd); err != nil {
			return nil, fmt.Errorf("failed to apply configu to Radix deployment: %w", err)
		}
	}

	return targetRd, nil
}

// type envDeployConfig struct {
// 	appExternalDNSAliases []radixv1.ExternalAlias
// 	activeRadixDeployment *radixv1.RadixDeployment
// }

// // Deploy Handles deploy step of the pipeline
// func (cli *DeployConfigStepImplementation) deploy(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
// 	appName := cli.GetAppName()
// 	log.Ctx(ctx).Info().Msgf("Deploying app config %s", appName)

// 	envConfigToDeploy, err := cli.getEnvConfigToDeploy(ctx, pipelineInfo)
// 	if len(envConfigToDeploy) == 0 {
// 		log.Ctx(ctx).Info().Msg("no environments to deploy to")
// 		return err
// 	}
// 	deployedEnvs, notDeployedEnvs, err := cli.deployEnvs(ctx, pipelineInfo, envConfigToDeploy)
// 	if len(deployedEnvs) == 0 {
// 		return err
// 	}
// 	log.Ctx(ctx).Info().Msgf("Deployed app config %s to environments %v", appName, strings.Join(deployedEnvs, ","))
// 	if err != nil {
// 		// some environments failed to deploy, but because some of them where - just log an error and not deployed envs.
// 		log.Ctx(ctx).Error().Err(err).Msgf("Failed to deploy app config %s to environments %v", appName, strings.Join(notDeployedEnvs, ","))
// 	}
// 	return nil
// }

// func (cli *DeployConfigStepImplementation) deployEnvs(ctx context.Context, pipelineInfo *model.PipelineInfo, envConfigToDeploy map[string]envDeployConfig) ([]string, []string, error) {
// 	deployedEnvCh, notDeployedEnvCh := make(chan string), make(chan string)
// 	errsCh, done := make(chan error), make(chan bool)
// 	defer close(deployedEnvCh)
// 	defer close(notDeployedEnvCh)
// 	defer close(errsCh)
// 	defer close(done)
// 	var notDeployedEnvs, deployedEnvs []string
// 	var errs []error
// 	go func() {
// 		for {
// 			select {
// 			case envName := <-deployedEnvCh:
// 				deployedEnvs = append(deployedEnvs, envName)
// 			case envName := <-notDeployedEnvCh:
// 				notDeployedEnvs = append(notDeployedEnvs, envName)
// 			case err := <-errsCh:
// 				errs = append(errs, err)
// 			case <-done:
// 				return
// 			}
// 		}
// 	}()

// 	var wg sync.WaitGroup
// 	appName := cli.GetAppName()
// 	for env, deployConfig := range envConfigToDeploy {
// 		wg.Add(1)
// 		go func(envName string) {
// 			defer wg.Done()
// 			if err := cli.deployToEnv(ctx, appName, envName, deployConfig, pipelineInfo); err != nil {
// 				errsCh <- err
// 				notDeployedEnvCh <- envName
// 			} else {
// 				deployedEnvCh <- envName
// 			}
// 		}(env)
// 	}
// 	wg.Wait()
// 	done <- true
// 	return deployedEnvs, notDeployedEnvs, errors.Join(errs...)
// }

// func (cli *DeployConfigStepImplementation) deployToEnv(ctx context.Context, appName, envName string, deployConfig envDeployConfig, pipelineInfo *model.PipelineInfo) error {
// 	defaultEnvVars := getDefaultEnvVars(pipelineInfo)

// 	if commitID, ok := defaultEnvVars[defaults.RadixCommitHashEnvironmentVariable]; !ok || len(commitID) == 0 {
// 		defaultEnvVars[defaults.RadixCommitHashEnvironmentVariable] = pipelineInfo.PipelineArguments.CommitID // Commit ID specified by job arguments
// 	}

// 	radixDeployment, err := constructForTargetEnvironment(appName, envName, pipelineInfo, deployConfig)

// 	if err != nil {
// 		return fmt.Errorf("failed to create Radix deployment in environment %s. %w", envName, err)
// 	}

// 	namespace := utils.GetEnvironmentNamespace(cli.GetAppName(), envName)
// 	if err = cli.namespaceWatcher.WaitFor(ctx, namespace); err != nil {
// 		return fmt.Errorf("failed to get environment namespace %s, for app %s. %w", namespace, appName, err)
// 	}

// 	radixDeploymentName := radixDeployment.GetName()
// 	log.Ctx(ctx).Info().Msgf("Apply Radix deployment %s to environment %s", radixDeploymentName, envName)
// 	if _, err = cli.GetRadixclient().RadixV1().RadixDeployments(radixDeployment.GetNamespace()).Create(context.Background(), radixDeployment, metav1.CreateOptions{}); err != nil {
// 		return fmt.Errorf("failed to apply Radix deployment for app %s to environment %s. %w", appName, envName, err)
// 	}

// 	if err = cli.radixDeploymentWatcher.WaitForActive(ctx, namespace, radixDeploymentName); err != nil {
// 		log.Ctx(ctx).Error().Err(err).Msgf("Failed to activate Radix deployment %s in environment %s. Deleting deployment", radixDeploymentName, envName)
// 		if deleteErr := cli.GetRadixclient().RadixV1().RadixDeployments(radixDeployment.GetNamespace()).Delete(context.Background(), radixDeploymentName, metav1.DeleteOptions{}); deleteErr != nil && !k8serrors.IsNotFound(deleteErr) {
// 			log.Ctx(ctx).Error().Err(deleteErr).Msgf("Failed to delete Radix deployment")
// 		}
// 		return err
// 	}

// 	return nil
// }

// func constructForTargetEnvironment(appName, envName string, pipelineInfo *model.PipelineInfo, deployConfig envDeployConfig) (*radixv1.RadixDeployment, error) {
// 	radixDeployment := deployConfig.activeRadixDeployment.DeepCopy()
// 	if err := setMetadata(appName, envName, pipelineInfo, radixDeployment); err != nil {
// 		return nil, err
// 	}
// 	setExternalDNSesToRadixDeployment(radixDeployment, deployConfig)
// 	radixDeployment.Status = radixv1.RadixDeployStatus{}
// 	return radixDeployment, nil
// }

// func setMetadata(appName, envName string, pipelineInfo *model.PipelineInfo, radixDeployment *radixv1.RadixDeployment) error {
// 	radixConfigHash, err := internal.CreateRadixApplicationHash(pipelineInfo.RadixApplication)
// 	if err != nil {
// 		return err
// 	}
// 	buildSecretHash, err := internal.CreateBuildSecretHash(pipelineInfo.BuildSecret)
// 	if err != nil {
// 		return err
// 	}
// 	envVars := getDefaultEnvVars(pipelineInfo)
// 	commitID := envVars[defaults.RadixCommitHashEnvironmentVariable]
// 	gitTags := envVars[defaults.RadixGitTagsEnvironmentVariable]
// 	radixDeployment.ObjectMeta = metav1.ObjectMeta{
// 		Name:        utils.GetDeploymentName(appName, envName, pipelineInfo.PipelineArguments.ImageTag),
// 		Namespace:   utils.GetEnvironmentNamespace(appName, envName),
// 		Annotations: radixannotations.ForRadixDeployment(gitTags, commitID, buildSecretHash, radixConfigHash),
// 		Labels: radixlabels.Merge(
// 			radixlabels.ForApplicationName(appName),
// 			radixlabels.ForEnvironmentName(envName),
// 			radixlabels.ForCommitId(commitID),
// 			radixlabels.ForPipelineJobName(pipelineInfo.PipelineArguments.JobName),
// 			radixlabels.ForRadixImageTag(pipelineInfo.PipelineArguments.ImageTag),
// 		),
// 	}
// 	return nil
// }

// func setExternalDNSesToRadixDeployment(radixDeployment *radixv1.RadixDeployment, deployConfig envDeployConfig) {
// 	externalAliasesMap := slice.Reduce(deployConfig.appExternalDNSAliases, make(map[string][]radixv1.ExternalAlias), func(acc map[string][]radixv1.ExternalAlias, dnsExternalAlias radixv1.ExternalAlias) map[string][]radixv1.ExternalAlias {
// 		acc[dnsExternalAlias.Component] = append(acc[dnsExternalAlias.Component], dnsExternalAlias)
// 		return acc
// 	})
// 	for i := 0; i < len(radixDeployment.Spec.Components); i++ {
// 		radixDeployment.Spec.Components[i].ExternalDNS = slice.Map(externalAliasesMap[radixDeployment.Spec.Components[i].Name], func(externalAlias radixv1.ExternalAlias) radixv1.RadixDeployExternalDNS {
// 			return radixv1.RadixDeployExternalDNS{
// 				FQDN: externalAlias.Alias, UseCertificateAutomation: externalAlias.UseCertificateAutomation}
// 		})
// 	}
// }

// func (cli *DeployConfigStepImplementation) getEnvConfigToDeploy(ctx context.Context, pipelineInfo *model.PipelineInfo) (map[string]envDeployConfig, error) {
// 	envDeployConfigs := cli.getEnvDeployConfigs(pipelineInfo)
// 	if err := cli.setActiveRadixDeployments(ctx, envDeployConfigs); err != nil {
// 		return nil, err
// 	}
// 	return getApplicableEnvDeployConfig(ctx, envDeployConfigs, pipelineInfo), nil
// }

// type activeDeploymentExternalDNS struct {
// 	externalDNS   radixv1.RadixDeployExternalDNS
// 	componentName string
// }

// func getApplicableEnvDeployConfig(ctx context.Context, envDeployConfigs map[string]envDeployConfig, pipelineInfo *model.PipelineInfo) map[string]envDeployConfig {
// 	applicableEnvDeployConfig := make(map[string]envDeployConfig)

// 	for envName, deployConfig := range envDeployConfigs {
// 		appExternalDNSAliases := slice.FindAll(pipelineInfo.RadixApplication.Spec.DNSExternalAlias, func(dnsExternalAlias radixv1.ExternalAlias) bool { return dnsExternalAlias.Environment == envName })
// 		if deployConfig.activeRadixDeployment == nil {
// 			if len(appExternalDNSAliases) > 0 {
// 				log.Ctx(ctx).Info().Msgf("External DNS alias(es) exists for the environment %s, but it has no active Radix deployment yet - ignore DNS alias(es).", envName)
// 			}
// 			continue
// 		}
// 		activeDeploymentExternalDNSes := getActiveDeploymentExternalDNSes(deployConfig)
// 		if equalExternalDNSAliases(activeDeploymentExternalDNSes, appExternalDNSAliases) {
// 			continue
// 		}
// 		applicableEnvDeployConfig[envName] = envDeployConfig{
// 			appExternalDNSAliases: appExternalDNSAliases,
// 			activeRadixDeployment: deployConfig.activeRadixDeployment,
// 		}

// 	}
// 	return applicableEnvDeployConfig
// }

// func getActiveDeploymentExternalDNSes(deployConfig envDeployConfig) []activeDeploymentExternalDNS {
// 	return slice.Reduce(deployConfig.activeRadixDeployment.Spec.Components, make([]activeDeploymentExternalDNS, 0),
// 		func(acc []activeDeploymentExternalDNS, component radixv1.RadixDeployComponent) []activeDeploymentExternalDNS {
// 			externalDNSes := slice.Map(component.ExternalDNS, func(externalDNS radixv1.RadixDeployExternalDNS) activeDeploymentExternalDNS {
// 				return activeDeploymentExternalDNS{componentName: component.Name, externalDNS: externalDNS}
// 			})
// 			return append(acc, externalDNSes...)
// 		})
// }

// func equalExternalDNSAliases(activeDeploymentExternalDNSes []activeDeploymentExternalDNS, appExternalAliases []radixv1.ExternalAlias) bool {
// 	if len(activeDeploymentExternalDNSes) != len(appExternalAliases) {
// 		return false
// 	}
// 	appExternalAliasesMap := slice.Reduce(appExternalAliases, make(map[string]radixv1.ExternalAlias), func(acc map[string]radixv1.ExternalAlias, dnsExternalAlias radixv1.ExternalAlias) map[string]radixv1.ExternalAlias {
// 		acc[dnsExternalAlias.Alias] = dnsExternalAlias
// 		return acc
// 	})
// 	return slice.All(activeDeploymentExternalDNSes, func(activeExternalDNS activeDeploymentExternalDNS) bool {
// 		appExternalAlias, ok := appExternalAliasesMap[activeExternalDNS.externalDNS.FQDN]
// 		return ok &&
// 			appExternalAlias.UseCertificateAutomation == activeExternalDNS.externalDNS.UseCertificateAutomation &&
// 			appExternalAlias.Component == activeExternalDNS.componentName
// 	})
// }

// func (cli *DeployConfigStepImplementation) getEnvDeployConfigs(pipelineInfo *model.PipelineInfo) map[string]envDeployConfig {
// 	return slice.Reduce(pipelineInfo.RadixApplication.Spec.Environments, make(map[string]envDeployConfig), func(acc map[string]envDeployConfig, env radixv1.Environment) map[string]envDeployConfig {
// 		acc[env.Name] = envDeployConfig{}
// 		return acc
// 	})
// }

// func (cli *DeployConfigStepImplementation) setActiveRadixDeployments(ctx context.Context, envDeployConfigs map[string]envDeployConfig) error {
// 	var errs []error
// 	for envName, deployConfig := range envDeployConfigs {
// 		if activeRd, err := internal.GetActiveRadixDeployment(ctx, cli.GetKubeutil(), utils.GetEnvironmentNamespace(cli.GetAppName(), envName)); err != nil {
// 			errs = append(errs, err)
// 		} else if activeRd != nil {
// 			deployConfig.activeRadixDeployment = activeRd
// 		}
// 		envDeployConfigs[envName] = deployConfig
// 	}
// 	return errors.Join(errs...)
// }

// func getDefaultEnvVars(pipelineInfo *model.PipelineInfo) radixv1.EnvVarsMap {
// 	gitCommitHash := pipelineInfo.GitCommitHash
// 	gitTags := pipelineInfo.GitTags
// 	envVarsMap := make(radixv1.EnvVarsMap)
// 	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = gitCommitHash
// 	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = gitTags
// 	return envVarsMap
// }
