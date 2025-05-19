package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/equinor/radix-operator/pipeline-runner/internal/runner"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/utils/logger"
	dnsaliasconfig "github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/git"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var overrideUseBuildCache, refreshBuildCache model.BoolPtr

// Requirements to run, pipeline must have:
// - access to create Jobs in "app" namespace it runs under
// - access to create RD in all namespaces
// - a secret git-ssh-keys containing deployment key to git repo provided in RR
// - a secret radix-sp-acr-azure with credentials to access our private ACR
// - a secret radix-snyk-service-account with access token to SNYK service account

func main() {
	pipelineArgs := &model.PipelineArguments{
		DNSConfig: &dnsaliasconfig.DNSConfig{ReservedAppDNSAliases: make(map[string]string)},
	}
	logger.InitLogger(pipelineArgs.LogLevel)

	cmd := &cobra.Command{
		Use: "run",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancelCtx := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
			defer cancelCtx()

			runner, err := prepareRunner(ctx, pipelineArgs)
			if err != nil {
				log.Error().Err(err).Msg("Failed to prepare runner")
				os.Exit(1)
			}

			err = runner.Run(ctx)

			teardownCtx, cancelTeardownCtx := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancelTeardownCtx()

			if err != nil {
				os.Exit(2)
			}

			err = runner.CreateResultConfigMap(teardownCtx)
			if err != nil {
				log.Error().Err(err).Msg("Failed to create result ConfigMap")
				os.Exit(3)
			}

			os.Exit(0)
		},
	}

	err := setPipelineArgsFromArguments(cmd, pipelineArgs, os.Args[1:])
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse args")
		os.Exit(1)
	}

	cmd.Run(nil, nil)
}

// runs os.Exit(1) if error
func prepareRunner(ctx context.Context, pipelineArgs *model.PipelineArguments) (*runner.PipelineRunner, error) {
	client, radixClient, kedaClient, prometheusOperatorClient, secretProviderClient, _, tektonClient := utils.GetKubernetesClient(ctx)

	pipelineDefinition, err := pipeline.GetPipelineFromName(pipelineArgs.PipelineType)
	if err != nil {
		return nil, err
	}

	pipelineRunner := runner.NewRunner(client, radixClient, kedaClient, prometheusOperatorClient, secretProviderClient, tektonClient, pipelineDefinition, pipelineArgs.AppName)

	err = pipelineRunner.PrepareRun(ctx, pipelineArgs)
	if err != nil {
		return nil, err
	}

	return &pipelineRunner, err
}

func setPipelineArgsFromArguments(cmd *cobra.Command, pipelineArgs *model.PipelineArguments, arguments []string) error {
	cmd.Flags().StringVar(&pipelineArgs.AppName, defaults.RadixAppEnvironmentVariable, "", "Radix application name")
	cmd.Flags().StringVar(&pipelineArgs.JobName, defaults.RadixPipelineJobEnvironmentVariable, "", "Pipeline job name")
	cmd.Flags().StringVar(&pipelineArgs.PipelineType, defaults.RadixPipelineTypeEnvironmentVariable, "", "Pipeline type")
	cmd.Flags().StringVar(&pipelineArgs.Branch, defaults.RadixBranchEnvironmentVariable, "", "Branch to deploy to")
	cmd.Flags().StringVar(&pipelineArgs.GitEventRefsType, defaults.RadixGitEventRefsTypeEnvironmentVariable, "branch", "Git event refs type")
	cmd.Flags().StringVar(&pipelineArgs.CommitID, defaults.RadixCommitIdEnvironmentVariable, "", "Commit ID to build from")
	cmd.Flags().StringVar(&pipelineArgs.DeploymentName, defaults.RadixPromoteDeploymentEnvironmentVariable, "", "Radix deployment name")
	cmd.Flags().StringVar(&pipelineArgs.FromEnvironment, defaults.RadixPromoteFromEnvironmentEnvironmentVariable, "", "Radix application environment name to promote from")
	cmd.Flags().StringVar(&pipelineArgs.ToEnvironment, defaults.RadixPipelineJobToEnvironmentEnvironmentVariable, "", "Radix application environment name to build-deploy or promote to")
	cmd.Flags().StringVar(&pipelineArgs.ImageBuilder, defaults.RadixImageBuilderEnvironmentVariable, "", "Radix Image Builder docker image")
	cmd.Flags().StringVar(&pipelineArgs.BuildKitImageBuilder, defaults.RadixBuildKitImageBuilderEnvironmentVariable, "", "Radix Build Kit Image Builder container image")
	cmd.Flags().StringVar(&pipelineArgs.SeccompProfileFileName, defaults.SeccompProfileFileNameEnvironmentVariable, "", "Filename of the seccomp profile injected by daemonset, relative to the /var/lib/kubelet/seccomp directory on node")
	cmd.Flags().StringVar(&pipelineArgs.Clustertype, defaults.RadixClusterTypeEnvironmentVariable, "", "Cluster type")
	cmd.Flags().StringVar(&pipelineArgs.Clustername, defaults.ClusternameEnvironmentVariable, "", "Cluster name")
	cmd.Flags().StringVar(&pipelineArgs.ContainerRegistry, defaults.ContainerRegistryEnvironmentVariable, "", "Container registry")
	cmd.Flags().StringVar(&pipelineArgs.AppContainerRegistry, defaults.AppContainerRegistryEnvironmentVariable, "", "App Container registry")
	cmd.Flags().StringVar(&pipelineArgs.SubscriptionId, defaults.AzureSubscriptionIdEnvironmentVariable, "", "Azure Subscription ID")
	cmd.Flags().StringVar(&pipelineArgs.RadixZone, defaults.RadixZoneEnvironmentVariable, "", "Radix zone")
	cmd.Flags().StringVar(&pipelineArgs.RadixConfigFile, defaults.RadixConfigFileEnvironmentVariable, "", "Radix config file name. Example: radixconfig.yaml")
	cmd.Flags().StringVar(&pipelineArgs.ImageTag, defaults.RadixImageTagEnvironmentVariable, "latest", "Docker image tag")
	cmd.Flags().StringVar(&pipelineArgs.LogLevel, defaults.LogLevel, "INFO", "Log level: ERROR, WARN, INFO (default), DEBUG")
	cmd.Flags().StringVar(&pipelineArgs.Builder.ResourcesLimitsMemory, defaults.OperatorAppBuilderResourcesLimitsMemoryEnvironmentVariable, "2000M", "Image builder resource limit memory")
	cmd.Flags().StringVar(&pipelineArgs.Builder.ResourcesLimitsCPU, defaults.OperatorAppBuilderResourcesLimitsCPUEnvironmentVariable, "1000m", "Image builder resource limit CPU")
	cmd.Flags().StringVar(&pipelineArgs.Builder.ResourcesRequestsCPU, defaults.OperatorAppBuilderResourcesRequestsCPUEnvironmentVariable, "200m", "Image builder resource requests CPU")
	cmd.Flags().StringVar(&pipelineArgs.Builder.ResourcesRequestsMemory, defaults.OperatorAppBuilderResourcesRequestsMemoryEnvironmentVariable, "500M", "Image builder resource requests memory")
	cmd.Flags().StringVar(&pipelineArgs.ExternalContainerRegistryDefaultAuthSecret, defaults.RadixExternalRegistryDefaultAuthEnvironmentVariable, "", "Name of secret of type `kubernetes.io/dockerconfigjson` containign default credentials for external container registries")
	cmd.Flags().Var(&overrideUseBuildCache, defaults.RadixOverrideUseBuildCacheEnvironmentVariable, "Optional. Overrides configured or default useBuildCache option. It is applicable when the useBuildKit option is set as true.")
	cmd.Flags().Var(&refreshBuildCache, defaults.RadixRefreshBuildCacheEnvironmentVariable, "Optional. Forces to rebuild cache when useBuildKit and useBuildCache or overrideUseBuildCache are true.")
	var pushImage string
	cmd.Flags().StringVar(&pushImage, defaults.RadixPushImageEnvironmentVariable, "0", "Push docker image to a repository")
	var debug string
	cmd.Flags().StringVar(&debug, "DEBUG", "false", "Debug information")
	cmd.Flags().StringToStringVar(&pipelineArgs.ImageTagNames, defaults.RadixImageTagNameEnvironmentVariable, make(map[string]string), "Image tag names for components (optional)")
	cmd.Flags().StringToStringVar(&pipelineArgs.DNSConfig.ReservedAppDNSAliases, defaults.RadixReservedAppDNSAliasesEnvironmentVariable, make(map[string]string), "The list of DNS aliases, reserved for Radix platform Radix application")
	cmd.Flags().StringSliceVar(&pipelineArgs.DNSConfig.ReservedDNSAliases, defaults.RadixReservedDNSAliasesEnvironmentVariable, make([]string, 0), "The list of DNS aliases, reserved for Radix platform services")
	cmd.Flags().StringSliceVar(&pipelineArgs.ComponentsToDeploy, defaults.RadixComponentsToDeployVariable, make([]string, 0), "The list of components to deploy (optional)")
	// Git clone init container images
	cmd.Flags().StringVar(&pipelineArgs.GitCloneNsLookupImage, defaults.RadixGitCloneNsLookupImageEnvironmentVariable, "alpine:latest", "Container image with nslookup used by git clone init containers")
	cmd.Flags().StringVar(&pipelineArgs.GitCloneGitImage, defaults.RadixGitCloneGitImageEnvironmentVariable, "alpine/git:latest", "Container image with git used by git clone init containers")
	cmd.Flags().StringVar(&pipelineArgs.GitCloneBashImage, defaults.RadixGitCloneBashImageEnvironmentVariable, "bash:latest", "Container image with bash used by git clone init containers")
	cmd.Flags().BoolVar(&pipelineArgs.ApplyConfigOptions.DeployExternalDNS, defaults.RadixPipelineApplyConfigDeployExternalDNSFlag, false, "Deploy changes to External DNS configuration with the 'apply-config' pipeline")
	cmd.Flags().StringVar(&pipelineArgs.GitWorkspace, defaults.RadixGithubWorkspaceEnvironmentVariable, git.Workspace, fmt.Sprintf("(Optional) Workspace path to the cloned GitHub repository. Default %s", git.Workspace))
	cmd.Flags().BoolVar(&pipelineArgs.TriggeredFromWebhook, defaults.RadixPipelineJobTriggeredFromWebhookEnvironmentVariable, false, "Indicates if the pipeline was triggered from a webhook")

	err := cmd.Flags().Parse(arguments)
	if err != nil {
		return fmt.Errorf("failed to parse command arguments. Error: %v", err)
	}
	if len(pipelineArgs.DNSConfig.ReservedAppDNSAliases) == 0 {
		return fmt.Errorf("missing DNS aliases, reserved for Radix platform Radix application")
	}
	if len(pipelineArgs.DNSConfig.ReservedDNSAliases) == 0 {
		return fmt.Errorf("missing DNS aliases, reserved for Radix platform services")
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
