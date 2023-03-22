package main

import (
	"os"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	pipe "github.com/equinor/radix-operator/pipeline-runner/pipelines"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// Requirements to run, pipeline must have:
// - access to create Jobs in "app" namespace it runs under
// - access to create RD in all namespaces
// - a secret git-ssh-keys containing deployment key to git repo provided in RR
// - a secret radix-sp-acr-azure with credentials to access our private ACR
// - a secret radix-snyk-service-account with access token to SNYK service account

func main() {
	pipelineArgs := &model.PipelineArguments{}

	cmd := &cobra.Command{
		Use: "run",
		Run: func(cmd *cobra.Command, args []string) {

			runner, err := prepareRunner(pipelineArgs)
			if err != nil {
				log.Error(err.Error())
				os.Exit(1)
			}

			err = runner.Run()
			runner.TearDown()
			if err != nil {
				os.Exit(2)
			}

			err = runner.CreateResultConfigMap()
			if err != nil {
				log.Error(err.Error())
				os.Exit(3)
			}

			os.Exit(0)
		},
	}

	setPipelineArgsFromArguments(cmd, pipelineArgs, os.Args[1:])

	setLogLevel(pipelineArgs.LogLevel)

	cmd.Run(nil, nil)
}

// runs os.Exit(1) if error
func prepareRunner(pipelineArgs *model.PipelineArguments) (*pipe.PipelineRunner, error) {
	client, radixClient, prometheusOperatorClient, secretProviderClient := utils.GetKubernetesClient()

	pipelineDefinition, err := pipeline.GetPipelineFromName(pipelineArgs.PipelineType)
	if err != nil {
		return nil, err
	}

	pipelineRunner := pipe.InitRunner(client, radixClient, prometheusOperatorClient, secretProviderClient, pipelineDefinition, pipelineArgs.AppName)

	err = pipelineRunner.PrepareRun(pipelineArgs)
	if err != nil {
		return nil, err
	}

	return &pipelineRunner, err
}

func setPipelineArgsFromArguments(cmd *cobra.Command, pipelineArgs *model.PipelineArguments, arguments []string) {
	cmd.Flags().StringVar(&pipelineArgs.AppName, defaults.RadixAppEnvironmentVariable, "", "Radix application name")
	cmd.Flags().StringVar(&pipelineArgs.JobName, defaults.RadixPipelineJobEnvironmentVariable, "", "Pipeline job name")
	cmd.Flags().StringVar(&pipelineArgs.PipelineType, defaults.RadixPipelineTypeEnvironmentVariable, "", "Pipeline type")
	cmd.Flags().StringVar(&pipelineArgs.Branch, defaults.RadixBranchEnvironmentVariable, "", "Branch to deploy to")
	cmd.Flags().StringVar(&pipelineArgs.CommitID, defaults.RadixCommitIdEnvironmentVariable, "", "Commit ID to build from")
	cmd.Flags().StringVar(&pipelineArgs.DeploymentName, defaults.RadixPromoteDeploymentEnvironmentVariable, "", "Radix deployment name")
	cmd.Flags().StringVar(&pipelineArgs.FromEnvironment, defaults.RadixPromoteFromEnvironmentEnvironmentVariable, "", "Radix application environment name to promote from")
	cmd.Flags().StringVar(&pipelineArgs.ToEnvironment, defaults.RadixPromoteToEnvironmentEnvironmentVariable, "", "Radix application environment name to promote to")
	cmd.Flags().StringVar(&pipelineArgs.TektonPipeline, defaults.RadixTektonPipelineImageEnvironmentVariable, "", "Radix Tekton docker image")
	cmd.Flags().StringVar(&pipelineArgs.ImageBuilder, defaults.RadixImageBuilderEnvironmentVariable, "", "Radix Image Builder docker image")
	cmd.Flags().StringVar(&pipelineArgs.Clustertype, defaults.RadixClusterTypeEnvironmentVariable, "", "Cluster type")
	cmd.Flags().StringVar(&pipelineArgs.Clustername, defaults.ClusternameEnvironmentVariable, "", "Cluster name")
	cmd.Flags().StringVar(&pipelineArgs.ContainerRegistry, defaults.ContainerRegistryEnvironmentVariable, "", "Container registry")
	cmd.Flags().StringVar(&pipelineArgs.SubscriptionId, defaults.AzureSubscriptionIdEnvironmentVariable, "", "Azure Subscription ID")
	cmd.Flags().StringVar(&pipelineArgs.RadixZone, defaults.RadixZoneEnvironmentVariable, "", "Radix zone")
	cmd.Flags().StringVar(&pipelineArgs.RadixConfigFile, defaults.RadixConfigFileEnvironmentVariable, "", "Radix config file name. Example: /workspace/radixconfig.yaml")
	cmd.Flags().StringVar(&pipelineArgs.ImageTag, defaults.RadixImageTagEnvironmentVariable, "latest", "Docker image tag")
	cmd.Flags().StringVar(&pipelineArgs.LogLevel, defaults.LogLevel, "INFO", "Log level: ERROR, INFO (default), DEBUG")
	cmd.Flags().BoolVar(&pipelineArgs.UseCache, defaults.RadixUseCacheEnvironmentVariable, false, "Use cache")
	cmd.Flags().BoolVar(&pipelineArgs.PushImage, defaults.RadixPushImageEnvironmentVariable, false, "Push docker image to a repository")
	cmd.Flags().BoolVar(&pipelineArgs.Debug, "DEBUG", false, "Debug information")
	cmd.Flags().StringToStringVar(&pipelineArgs.ImageTagNames, defaults.RadixImageTagNameEnvironmentVariable, make(map[string]string), "Image tag names for components (optional)")

	cmd.Flags().Parse(arguments)

	if pipelineArgs.ImageTagNames == nil || len(pipelineArgs.ImageTagNames) == 0 {
		log.Infoln("Image tag names provided:")
		for componentName, imageTagName := range pipelineArgs.ImageTagNames {
			log.Infof("- %s:%s", componentName, imageTagName)
		}
	}

	pipelineArgs.PushImage = pipelineArgs.PipelineType == string(v1.BuildDeploy) || pipelineArgs.PushImage // build and deploy require push
}

func setLogLevel(logLevel string) {
	switch logLevel {
	case "DEBUG":
		log.SetLevel(log.DebugLevel)
	case "ERROR":
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}
}
