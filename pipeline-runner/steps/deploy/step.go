package deploy

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pipeline-runner/internal/watcher"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeployStepImplementation Step to deploy RD into environment
type DeployStepImplementation struct {
	stepType               pipeline.StepType
	namespaceWatcher       watcher.NamespaceWatcher
	radixDeploymentWatcher watcher.RadixDeploymentWatcher
	model.DefaultStepImplementation
}

// NewDeployStep Constructor
func NewDeployStep(namespaceWatcher watcher.NamespaceWatcher, radixDeploymentWatcher watcher.RadixDeploymentWatcher) model.Step {
	return &DeployStepImplementation{
		stepType:               pipeline.DeployStep,
		namespaceWatcher:       namespaceWatcher,
		radixDeploymentWatcher: radixDeploymentWatcher,
	}
}

// ImplementationForType Override of default step method
func (cli *DeployStepImplementation) ImplementationForType() pipeline.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *DeployStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Succeded: deploy application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *DeployStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed to deploy application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *DeployStepImplementation) Run(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
	err := cli.deploy(ctx, pipelineInfo)
	return err
}

// Deploy Handles deploy step of the pipeline
func (cli *DeployStepImplementation) deploy(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
	appName := cli.GetAppName()
	log.Ctx(ctx).Info().Msgf("Deploying app %s", appName)

	if len(pipelineInfo.TargetEnvironments) == 0 {
		log.Ctx(ctx).Info().Msgf("skip deploy step as branch %s is not mapped to any environment", pipelineInfo.PipelineArguments.Branch)
		return nil
	}

	for _, env := range pipelineInfo.TargetEnvironments {
		if err := cli.deployToEnv(ctx, appName, env, pipelineInfo); err != nil {
			return err
		}
	}
	return nil
}

func (cli *DeployStepImplementation) deployToEnv(ctx context.Context, appName, envName string, pipelineInfo *model.PipelineInfo) error {
	defaultEnvVars, err := getDefaultEnvVars(pipelineInfo)
	if err != nil {
		return fmt.Errorf("failed to retrieve default env vars for RadixDeployment in app  %s. %v", appName, err)
	}

	if commitID, ok := defaultEnvVars[defaults.RadixCommitHashEnvironmentVariable]; !ok || len(commitID) == 0 {
		defaultEnvVars[defaults.RadixCommitHashEnvironmentVariable] = pipelineInfo.PipelineArguments.CommitID // Commit ID specified by job arguments
	}

	radixApplicationHash, err := internal.CreateRadixApplicationHash(pipelineInfo.RadixApplication)
	if err != nil {
		return err
	}

	buildSecretHash, err := internal.CreateBuildSecretHash(pipelineInfo.BuildSecret)
	if err != nil {
		return err
	}

	activeRd, err := internal.GetActiveRadixDeployment(ctx, cli.GetKubeutil(), utils.GetEnvironmentNamespace(appName, envName))
	if err != nil {
		return err
	}
	radixDeployment, err := internal.ConstructForTargetEnvironment(
		ctx,
		pipelineInfo.RadixApplication,
		activeRd,
		pipelineInfo.PipelineArguments.JobName,
		pipelineInfo.PipelineArguments.ImageTag,
		pipelineInfo.PipelineArguments.Branch,
		pipelineInfo.DeployEnvironmentComponentImages[envName],
		envName,
		defaultEnvVars,
		radixApplicationHash,
		buildSecretHash,
		pipelineInfo.PrepareBuildContext,
		pipelineInfo.PipelineArguments.ComponentsToDeploy)

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

	if err := cli.radixDeploymentWatcher.WaitForActive(ctx, namespace, radixDeploymentName); err != nil {
		log.Ctx(ctx).Error().Err(err).Msgf("Failed to activate Radix deployment %s in environment %s. Deleting deployment", radixDeploymentName, envName)
		if err := cli.GetRadixclient().RadixV1().RadixDeployments(radixDeployment.GetNamespace()).Delete(context.Background(), radixDeploymentName, metav1.DeleteOptions{}); err != nil {
			log.Ctx(ctx).Error().Err(err).Msgf("Failed to delete Radix deployment")
		}
		return err
	}
	return nil
}

func getDefaultEnvVars(pipelineInfo *model.PipelineInfo) (radixv1.EnvVarsMap, error) {
	gitCommitHash := pipelineInfo.GitCommitHash
	gitTags := pipelineInfo.GitTags

	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = gitCommitHash
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = gitTags

	return envVarsMap, nil
}
