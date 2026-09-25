package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/equinor/radix-operator/pipeline-runner/flags"
	"github.com/equinor/radix-operator/pipeline-runner/internal/runner"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/utils/logger"
	"github.com/equinor/radix-operator/pkg/apis/git"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/scheme"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

// Requirements to run, pipeline must have:
// - access to create Jobs in "app" namespace it runs under
// - access to create RD in all namespaces
// - a secret git-ssh-keys containing deployment key to git repo provided in RR
// - a secret radix-sp-acr-azure with credentials to access our private ACR
// - a secret radix-snyk-service-account with access token to SNYK service account

func main() {
	pipelineArgs := model.PipelineArguments{}
	logger.InitLogger(pipelineArgs.LogLevel)

	cmd := &cobra.Command{
		Use: "run",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancelCtx := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
			defer cancelCtx()

			cli, err := prepareRunner(ctx, pipelineArgs)
			if err != nil {
				log.Error().Err(err).Msg("Failed to prepare runner")
				os.Exit(1)
			}

			err = cli.Run(ctx)
			if err != nil {
				os.Exit(2)
			}

			os.Exit(0)
		},
	}

	err := setPipelineArgsFromArguments(cmd, &pipelineArgs, os.Args[1:])
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse args")
		os.Exit(1)
	}

	cmd.Run(nil, nil)
}

func prepareRunner(ctx context.Context, pipelineArgs model.PipelineArguments) (*runner.PipelineRunner, error) {
	kubeclient, radixClient, _, _, _, tektonClient := utils.GetKubernetesClient()

	cfg := k8sconfig.GetConfigOrDie()
	cfg.WarningHandler = utils.ZerologWarningHandlerAdapter(log.Warn)
	dynamicClient, err := client.New(cfg, client.Options{Scheme: scheme.NewScheme()})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize dynamic client: %w", err)
	}

	pipelineDefinition, err := pipeline.GetPipelineFromName(pipelineArgs.PipelineType)
	if err != nil {
		return nil, err
	}

	pipelineRunner := runner.NewRunner(kubeclient, radixClient, dynamicClient, tektonClient, pipelineDefinition, pipelineArgs.AppName)

	err = pipelineRunner.PrepareRun(ctx, pipelineArgs)
	if err != nil {
		return nil, err
	}

	return &pipelineRunner, err
}

func setPipelineArgsFromArguments(cmd *cobra.Command, pipelineArgs *model.PipelineArguments, arguments []string) error {
	cmd.Flags().StringVar(&pipelineArgs.AppName, flags.AppName, "", "Radix application name")
	cmd.Flags().StringVar(&pipelineArgs.JobName, flags.JobName, "", "Pipeline job name")
	cmd.Flags().StringVar(&pipelineArgs.PipelineType, flags.PipelineType, "", "Pipeline type")
	cmd.Flags().StringVar(&pipelineArgs.Branch, flags.Branch, "", "Branch to deploy to. Deprecated - use GIT_REF instead") //nolint:staticcheck
	cmd.Flags().StringVar(&pipelineArgs.GitRef, flags.GitRef, "", "Branch or tag to build from")
	cmd.Flags().StringVar(&pipelineArgs.GitRefType, flags.GitRefType, "", "Git ref type")
	cmd.Flags().StringVar(&pipelineArgs.CommitID, flags.CommitID, "", "Commit ID to build from")
	cmd.Flags().StringVar(&pipelineArgs.PromoteDeploymentName, flags.PromoteDeploymentName, "", "Radix deployment name to promote")
	cmd.Flags().StringVar(&pipelineArgs.PromoteFromEnvironment, flags.PromoteFromEnvironment, "", "Radix application environment name to promote from")
	cmd.Flags().StringVar(&pipelineArgs.ToEnvironment, flags.ToEnvironment, "", "Radix application environment name to build-deploy or promote to")
	cmd.Flags().StringVar(&pipelineArgs.RadixConfigFile, flags.RadixConfigFile, "", "Radix config file name. Example: radixconfig.yaml")
	cmd.Flags().StringVar(&pipelineArgs.ImageTag, flags.ImageTag, "latest", "Docker image tag")
	cmd.Flags().StringVar(&pipelineArgs.LogLevel, flags.LogLevel, "INFO", "Log level: ERROR, WARN, INFO (default), DEBUG")
	cmd.Flags().StringToStringVar(&pipelineArgs.ImageTagNames, flags.ComponentsImageTagName, make(map[string]string), "Image tag names for components (optional)")
	cmd.Flags().StringSliceVar(&pipelineArgs.ComponentsToDeploy, flags.ComponentsToDeploy, make([]string, 0), "The list of components to deploy (optional)")
	cmd.Flags().BoolVar(&pipelineArgs.ApplyConfigOptions.DeployExternalDNS, flags.ApplyConfigDeployExternalDNS, false, "Deploy changes to External DNS configuration with the 'apply-config' pipeline")
	cmd.Flags().StringVar(&pipelineArgs.GitWorkspace, flags.GitWorkspace, git.Workspace, fmt.Sprintf("(Optional) Workspace path to the cloned GitHub repository. Default %s", git.Workspace))
	cmd.Flags().BoolVar(&pipelineArgs.TriggeredFromWebhook, flags.TriggeredFromWebhook, false, "Indicates if the pipeline was triggered from a webhook")
	cmd.Flags().StringVar(&pipelineArgs.ConfigMapName, flags.ConfigMapName, "", "Config map name containing the pipeline configuration")
	cmd.Flags().StringVar(&pipelineArgs.ConfigMapNamespace, flags.ConfigMapNamespace, "", "Config map namespace containing the pipeline configuration")
	cmd.Flags().StringVar(&pipelineArgs.ConfigMapKey, flags.ConfigMapKey, "", "Config map key containing the pipeline configuration")

	var pushImage string
	cmd.Flags().StringVar(&pushImage, flags.PushImage, "0", "Push docker image to a repository")

	var overrideUseBuildCache model.BoolPtr
	cmd.Flags().Var(&overrideUseBuildCache, flags.OverrideUseBuildCache, "Optional. Overrides configured or default useBuildCache option. It is applicable when the useBuildKit option is set as true.")

	var refreshBuildCache model.BoolPtr
	cmd.Flags().Var(&refreshBuildCache, flags.RefreshBuildCache, "Optional. Forces to rebuild cache when useBuildKit and useBuildCache or overrideUseBuildCache are true.")

	var debug string
	cmd.Flags().StringVar(&debug, flags.Debug, "false", "Debug information")

	err := cmd.Flags().Parse(arguments)
	if err != nil {
		return fmt.Errorf("failed to parse command arguments: %w", err)
	}

	pipelineArgs.PushImage, _ = strconv.ParseBool(pushImage)
	pipelineArgs.PushImage = pipelineArgs.PipelineType == string(radixv1.BuildDeploy) || pipelineArgs.PushImage // build and deploy require push
	pipelineArgs.OverrideUseBuildCache = overrideUseBuildCache.Get()
	pipelineArgs.RefreshBuildCache = refreshBuildCache.Get()
	pipelineArgs.Debug, _ = strconv.ParseBool(debug)

	if len(pipelineArgs.ImageTagNames) > 0 {
		log.Info().Msg("Image tag names provided:")
		for componentName, imageTagName := range pipelineArgs.ImageTagNames {
			log.Info().Msgf("- %s:%s", componentName, imageTagName)
		}
	}
	return nil
}
