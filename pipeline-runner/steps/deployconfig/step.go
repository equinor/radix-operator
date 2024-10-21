package deployconfig

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/internal/watcher"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixannotations "github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/rs/zerolog/log"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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
	return cli.deploy(ctx, pipelineInfo)
}

type envDeployConfig struct {
	appExternalDNSAliases []radixv1.ExternalAlias
	activeRadixDeployment *radixv1.RadixDeployment
}

// Deploy Handles deploy step of the pipeline
func (cli *DeployConfigStepImplementation) deploy(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
	appName := cli.GetAppName()
	log.Ctx(ctx).Info().Msgf("Deploying app config %s", appName)

	envConfigToDeploy, err := cli.getEnvConfigToDeploy(ctx, pipelineInfo)
	if len(envConfigToDeploy) == 0 {
		log.Ctx(ctx).Info().Msg("no environments to deploy to")
		return err
	}
	deployedEnvs, notDeployedEnvs, err := cli.deployEnvs(ctx, pipelineInfo, envConfigToDeploy)
	if len(deployedEnvs) == 0 {
		return err
	}
	log.Ctx(ctx).Info().Msgf("Deployed app config %s to environments %v", appName, strings.Join(deployedEnvs, ","))
	if err != nil {
		// some environments failed to deploy, but because some of them where - just log an error and not deployed envs.
		log.Ctx(ctx).Error().Err(err).Msgf("Failed to deploy app config %s to environments %v", appName, strings.Join(notDeployedEnvs, ","))
	}
	return nil
}

func (cli *DeployConfigStepImplementation) deployEnvs(ctx context.Context, pipelineInfo *model.PipelineInfo, envConfigToDeploy map[string]envDeployConfig) ([]string, []string, error) {
	deployedEnvCh, notDeployedEnvCh := make(chan string), make(chan string)
	errsCh, done := make(chan error), make(chan bool)
	defer close(deployedEnvCh)
	defer close(notDeployedEnvCh)
	defer close(errsCh)
	defer close(done)
	var notDeployedEnvs, deployedEnvs []string
	var errs []error
	go func() {
		for {
			select {
			case envName := <-deployedEnvCh:
				deployedEnvs = append(deployedEnvs, envName)
			case envName := <-notDeployedEnvCh:
				notDeployedEnvs = append(notDeployedEnvs, envName)
			case err := <-errsCh:
				errs = append(errs, err)
			case <-done:
				return
			}
		}
	}()

	var wg sync.WaitGroup
	appName := cli.GetAppName()
	for env, deployConfig := range envConfigToDeploy {
		wg.Add(1)
		go func(envName string) {
			defer wg.Done()
			if err := cli.deployToEnv(ctx, appName, envName, deployConfig, pipelineInfo); err != nil {
				errsCh <- err
				notDeployedEnvCh <- envName
			} else {
				deployedEnvCh <- envName
			}
		}(env)
	}
	wg.Wait()
	done <- true
	return deployedEnvs, notDeployedEnvs, errors.Join(errs...)
}

func (cli *DeployConfigStepImplementation) deployToEnv(ctx context.Context, appName, envName string, deployConfig envDeployConfig, pipelineInfo *model.PipelineInfo) error {
	defaultEnvVars := getDefaultEnvVars(pipelineInfo)

	if commitID, ok := defaultEnvVars[defaults.RadixCommitHashEnvironmentVariable]; !ok || len(commitID) == 0 {
		defaultEnvVars[defaults.RadixCommitHashEnvironmentVariable] = pipelineInfo.PipelineArguments.CommitID // Commit ID specified by job arguments
	}

	radixDeployment, err := constructForTargetEnvironment(appName, envName, pipelineInfo, deployConfig)

	if err != nil {
		return fmt.Errorf("failed to create Radix deployment in environment %s. %w", envName, err)
	}

	namespace := utils.GetEnvironmentNamespace(cli.GetAppName(), envName)
	if err = cli.namespaceWatcher.WaitFor(ctx, namespace); err != nil {
		return fmt.Errorf("failed to get environment namespace %s, for app %s. %w", namespace, appName, err)
	}

	radixDeploymentName := radixDeployment.GetName()
	log.Ctx(ctx).Info().Msgf("Apply Radix deployment %s to environment %s", radixDeploymentName, envName)
	if _, err = cli.GetRadixclient().RadixV1().RadixDeployments(radixDeployment.GetNamespace()).Create(context.Background(), radixDeployment, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to apply Radix deployment for app %s to environment %s. %w", appName, envName, err)
	}

	if err = cli.radixDeploymentWatcher.WaitForActive(ctx, namespace, radixDeploymentName); err != nil {
		log.Ctx(ctx).Error().Err(err).Msgf("Failed to activate Radix deployment %s in environment %s. Deleting deployment", radixDeploymentName, envName)
		if deleteErr := cli.GetRadixclient().RadixV1().RadixDeployments(radixDeployment.GetNamespace()).Delete(context.Background(), radixDeploymentName, metav1.DeleteOptions{}); deleteErr != nil && !k8serrors.IsNotFound(deleteErr) {
			log.Ctx(ctx).Error().Err(deleteErr).Msgf("Failed to delete Radix deployment")
		}
		return err
	}

	return nil
}

func constructForTargetEnvironment(appName, envName string, pipelineInfo *model.PipelineInfo, deployConfig envDeployConfig) (*radixv1.RadixDeployment, error) {
	radixDeployment := deployConfig.activeRadixDeployment.DeepCopy()
	if err := setMetadata(appName, envName, pipelineInfo, radixDeployment); err != nil {
		return nil, err
	}
	setExternalDNSesToRadixDeployment(radixDeployment, deployConfig)
	radixDeployment.Status = radixv1.RadixDeployStatus{}
	return radixDeployment, nil
}

func setMetadata(appName, envName string, pipelineInfo *model.PipelineInfo, radixDeployment *radixv1.RadixDeployment) error {
	radixConfigHash, err := internal.CreateRadixApplicationHash(pipelineInfo.RadixApplication)
	if err != nil {
		return err
	}
	buildSecretHash, err := internal.CreateBuildSecretHash(pipelineInfo.BuildSecret)
	if err != nil {
		return err
	}
	envVars := getDefaultEnvVars(pipelineInfo)
	commitID := envVars[defaults.RadixCommitHashEnvironmentVariable]
	gitTags := envVars[defaults.RadixGitTagsEnvironmentVariable]
	radixDeployment.ObjectMeta = metav1.ObjectMeta{
		Name:        utils.GetDeploymentName(appName, envName, pipelineInfo.PipelineArguments.ImageTag),
		Namespace:   utils.GetEnvironmentNamespace(appName, envName),
		Annotations: radixannotations.ForRadixDeployment(gitTags, commitID, buildSecretHash, radixConfigHash),
		Labels: radixlabels.Merge(
			radixlabels.ForApplicationName(appName),
			radixlabels.ForEnvironmentName(envName),
			radixlabels.ForCommitId(commitID),
			radixlabels.ForPipelineJobName(pipelineInfo.PipelineArguments.JobName),
			radixlabels.ForRadixImageTag(pipelineInfo.PipelineArguments.ImageTag),
		),
	}
	return nil
}

func setExternalDNSesToRadixDeployment(radixDeployment *radixv1.RadixDeployment, deployConfig envDeployConfig) {
	externalAliasesMap := slice.Reduce(deployConfig.appExternalDNSAliases, make(map[string][]radixv1.ExternalAlias), func(acc map[string][]radixv1.ExternalAlias, dnsExternalAlias radixv1.ExternalAlias) map[string][]radixv1.ExternalAlias {
		acc[dnsExternalAlias.Component] = append(acc[dnsExternalAlias.Component], dnsExternalAlias)
		return acc
	})
	for i := 0; i < len(radixDeployment.Spec.Components); i++ {
		radixDeployment.Spec.Components[i].ExternalDNS = slice.Map(externalAliasesMap[radixDeployment.Spec.Components[i].Name], func(externalAlias radixv1.ExternalAlias) radixv1.RadixDeployExternalDNS {
			return radixv1.RadixDeployExternalDNS{
				FQDN: externalAlias.Alias, UseCertificateAutomation: externalAlias.UseCertificateAutomation}
		})
	}
}

func (cli *DeployConfigStepImplementation) getEnvConfigToDeploy(ctx context.Context, pipelineInfo *model.PipelineInfo) (map[string]envDeployConfig, error) {
	envDeployConfigs := cli.getEnvDeployConfigs(pipelineInfo)
	if err := cli.setActiveRadixDeployments(ctx, envDeployConfigs); err != nil {
		return nil, err
	}
	return getApplicableEnvDeployConfig(ctx, envDeployConfigs, pipelineInfo), nil
}

type activeDeploymentExternalDNS struct {
	externalDNS   radixv1.RadixDeployExternalDNS
	componentName string
}

func getApplicableEnvDeployConfig(ctx context.Context, envDeployConfigs map[string]envDeployConfig, pipelineInfo *model.PipelineInfo) map[string]envDeployConfig {
	applicableEnvDeployConfig := make(map[string]envDeployConfig)
	for envName, deployConfig := range envDeployConfigs {
		appExternalDNSAliases := slice.FindAll(pipelineInfo.RadixApplication.Spec.DNSExternalAlias, func(dnsExternalAlias radixv1.ExternalAlias) bool { return dnsExternalAlias.Environment == envName })
		if deployConfig.activeRadixDeployment == nil {
			if len(appExternalDNSAliases) > 0 {
				log.Ctx(ctx).Info().Msgf("External DNS alias(es) exists for the environment %s, but it has no active Radix deployment yet - ignore DNS alias(es).", envName)
			}
			continue
		}
		activeDeploymentExternalDNSes := getActiveDeploymentExternalDNSes(deployConfig)
		if equalExternalDNSAliases(activeDeploymentExternalDNSes, appExternalDNSAliases) {
			continue
		}
		applicableEnvDeployConfig[envName] = envDeployConfig{
			appExternalDNSAliases: appExternalDNSAliases,
			activeRadixDeployment: deployConfig.activeRadixDeployment,
		}

	}
	return applicableEnvDeployConfig
}

func getActiveDeploymentExternalDNSes(deployConfig envDeployConfig) []activeDeploymentExternalDNS {
	return slice.Reduce(deployConfig.activeRadixDeployment.Spec.Components, make([]activeDeploymentExternalDNS, 0),
		func(acc []activeDeploymentExternalDNS, component radixv1.RadixDeployComponent) []activeDeploymentExternalDNS {
			externalDNSes := slice.Map(component.ExternalDNS, func(externalDNS radixv1.RadixDeployExternalDNS) activeDeploymentExternalDNS {
				return activeDeploymentExternalDNS{componentName: component.Name, externalDNS: externalDNS}
			})
			return append(acc, externalDNSes...)
		})
}

func equalExternalDNSAliases(activeDeploymentExternalDNSes []activeDeploymentExternalDNS, appExternalAliases []radixv1.ExternalAlias) bool {
	if len(activeDeploymentExternalDNSes) != len(appExternalAliases) {
		return false
	}
	appExternalAliasesMap := slice.Reduce(appExternalAliases, make(map[string]radixv1.ExternalAlias), func(acc map[string]radixv1.ExternalAlias, dnsExternalAlias radixv1.ExternalAlias) map[string]radixv1.ExternalAlias {
		acc[dnsExternalAlias.Alias] = dnsExternalAlias
		return acc
	})
	return slice.All(activeDeploymentExternalDNSes, func(activeExternalDNS activeDeploymentExternalDNS) bool {
		appExternalAlias, ok := appExternalAliasesMap[activeExternalDNS.externalDNS.FQDN]
		return ok &&
			appExternalAlias.UseCertificateAutomation == activeExternalDNS.externalDNS.UseCertificateAutomation &&
			appExternalAlias.Component == activeExternalDNS.componentName
	})
}

func (cli *DeployConfigStepImplementation) getEnvDeployConfigs(pipelineInfo *model.PipelineInfo) map[string]envDeployConfig {
	return slice.Reduce(pipelineInfo.RadixApplication.Spec.Environments, make(map[string]envDeployConfig), func(acc map[string]envDeployConfig, env radixv1.Environment) map[string]envDeployConfig {
		acc[env.Name] = envDeployConfig{}
		return acc
	})
}

func (cli *DeployConfigStepImplementation) setActiveRadixDeployments(ctx context.Context, envDeployConfigs map[string]envDeployConfig) error {
	var errs []error
	for envName, deployConfig := range envDeployConfigs {
		if activeRd, err := internal.GetActiveRadixDeployment(ctx, cli.GetKubeutil(), utils.GetEnvironmentNamespace(cli.GetAppName(), envName)); err != nil {
			errs = append(errs, err)
		} else if activeRd != nil {
			deployConfig.activeRadixDeployment = activeRd
		}
		envDeployConfigs[envName] = deployConfig
	}
	return errors.Join(errs...)
}

func getDefaultEnvVars(pipelineInfo *model.PipelineInfo) radixv1.EnvVarsMap {
	gitCommitHash := pipelineInfo.GitCommitHash
	gitTags := pipelineInfo.GitTags
	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = gitCommitHash
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = gitTags
	return envVarsMap
}
