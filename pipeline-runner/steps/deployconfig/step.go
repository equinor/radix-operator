package deploy

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
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
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
	err := cli.deploy(ctx, pipelineInfo)
	return err
}

type envDeployConfig struct {
	externalDNSAliases []radixv1.ExternalAlias
}

// Deploy Handles deploy step of the pipeline
func (cli *DeployConfigStepImplementation) deploy(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
	appName := cli.GetAppName()
	log.Ctx(ctx).Info().Msgf("Deploying app config %s", appName)

	envConfigToDeploy := cli.getEnvConfigToDeploy(pipelineInfo)
	if len(envConfigToDeploy) == 0 {
		log.Ctx(ctx).Info().Msg("skip deploy step")
		return nil
	}
	var deployedEnvs, notDeployedEnvs []string
	var errs []error
	var wg sync.WaitGroup
	var mu sync.Mutex
	for envName, envConfig := range envConfigToDeploy {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer mu.Unlock()
			if err := cli.deployToEnv(ctx, appName, envName, envConfig, pipelineInfo); err != nil {
				mu.Lock()
				errs = append(errs, err)
				notDeployedEnvs = append(notDeployedEnvs, envName)
			} else {
				mu.Lock()
				deployedEnvs = append(deployedEnvs, envName)
			}
		}()
	}
	wg.Wait()
	if len(deployedEnvs) == 0 {
		return errors.Join(errs...)
	}
	log.Ctx(ctx).Info().Msgf("Deployed app config %s to environments %v", appName, strings.Join(deployedEnvs, ","))
	if errs != nil {
		log.Ctx(ctx).Error().Err(errors.Join(errs...)).Msgf("Failed to deploy app config %s to environments %v", appName, strings.Join(notDeployedEnvs, ","))
	}
	return nil
}

func (cli *DeployConfigStepImplementation) deployToEnv(ctx context.Context, appName, envName string, envConfig envDeployConfig, pipelineInfo *model.PipelineInfo) error {
	defaultEnvVars := getDefaultEnvVars(pipelineInfo)

	if commitID, ok := defaultEnvVars[defaults.RadixCommitHashEnvironmentVariable]; !ok || len(commitID) == 0 {
		defaultEnvVars[defaults.RadixCommitHashEnvironmentVariable] = pipelineInfo.PipelineArguments.CommitID // Commit ID specified by job arguments
	}

	activeRd, err := internal.GetActiveRadixDeployment(ctx, cli.GetKubeutil(), utils.GetEnvironmentNamespace(appName, envName))
	if err != nil {
		return err
	}
	if activeRd == nil {
		return fmt.Errorf("failed to get active Radix deployment in environment %s - the config cannot be applied", envName)
	}
	radixDeployment, err := constructForTargetEnvironment(pipelineInfo, activeRd)

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

func constructForTargetEnvironment(pipelineInfo *model.PipelineInfo, activeRd *radixv1.RadixDeployment) (*radixv1.RadixDeployment, error) {
	envVars := getDefaultEnvVars(pipelineInfo)
	commitID := envVars[defaults.RadixCommitHashEnvironmentVariable]
	gitTags := envVars[defaults.RadixGitTagsEnvironmentVariable]
	radixDeployment := activeRd.DeepCopy()
	radixConfigHash, err := internal.CreateRadixApplicationHash(pipelineInfo.RadixApplication)
	if err != nil {
		return nil, err
	}
	buildSecretHash, err := internal.CreateBuildSecretHash(pipelineInfo.BuildSecret)
	if err != nil {
		return nil, err
	}
	radixDeployment.SetAnnotations(map[string]string{
		kube.RadixGitTagsAnnotation: gitTags,
		kube.RadixCommitAnnotation:  commitID,
		kube.RadixBuildSecretHash:   buildSecretHash,
		kube.RadixConfigHash:        radixConfigHash,
	})
	radixDeployment.ObjectMeta.Labels[kube.RadixCommitLabel] = commitID
	radixDeployment.ObjectMeta.Labels[kube.RadixCommitLabel] = commitID
	radixDeployment.ObjectMeta.Labels[kube.RadixJobNameLabel] = pipelineInfo.PipelineArguments.JobName
	return radixDeployment, nil
}

func (cli *DeployConfigStepImplementation) getEnvConfigToDeploy(pipelineInfo *model.PipelineInfo) map[string]envDeployConfig {
	accumulation := make(map[string]envDeployConfig)
	return slice.Reduce(pipelineInfo.RadixApplication.Spec.DNSExternalAlias, accumulation, func(acc map[string]envDeployConfig, alias radixv1.ExternalAlias) map[string]envDeployConfig {
		deployConfig, ok := acc[alias.Environment]
		if !ok {
			deployConfig = envDeployConfig{}
		}
		deployConfig.externalDNSAliases = append(deployConfig.externalDNSAliases, alias)
		acc[alias.Environment] = deployConfig
		return acc
	})
}

func getDefaultEnvVars(pipelineInfo *model.PipelineInfo) radixv1.EnvVarsMap {
	gitCommitHash := pipelineInfo.GitCommitHash
	gitTags := pipelineInfo.GitTags

	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = gitCommitHash
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = gitTags

	return envVarsMap
}
