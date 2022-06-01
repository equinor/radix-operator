package main

import (
	"os"
	"strings"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/model/env"
	pipe "github.com/equinor/radix-operator/pipeline-runner/pipelines"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// Requirements to run, pipeline must have:
// - access to create Jobs in "app" namespace it runs under
// - access to create RD in all namespaces
// - access to create new namespaces
// - a secret git-ssh-keys containing deployment key to git repo provided in RR
// - a secret radix-sp-acr-azure with credentials to access our private ACR
// - a secret radix-snyk-service-account with access token to SNYK service account

func main() {
	environment := env.NewEnvironment()
	setLogLevel(environment)

	runner, err := prepareRunner(environment)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	err = runner.Run()
	runner.TearDown()

	if err != nil {
		os.Exit(2)
	}

	os.Exit(0)
}

func setLogLevel(environment env.Env) {
	switch environment.GetLogLevel() {
	case string(env.LogLevelDebug):
		log.SetLevel(log.DebugLevel)
	case string(env.LogLevelError):
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}
}

// runs os.Exit(1) if error
func prepareRunner(environment env.Env) (*pipe.PipelineRunner, error) {
	args := getArgs()

	// Required when repo is not cloned
	appName := args[defaults.RadixAppEnvironmentVariable]

	pipelineArgs := model.GetPipelineArgsFromArguments(args)
	client, radixClient, prometheusOperatorClient, secretProviderClient := utils.GetKubernetesClient()

	pipelineDefinition, err := pipeline.GetPipelineFromName(pipelineArgs.PipelineType)
	if err != nil {
		return nil, err
	}

	pipelineRunner := pipe.InitRunner(client, radixClient, prometheusOperatorClient, secretProviderClient, pipelineDefinition, appName, environment)

	err = pipelineRunner.PrepareRun(pipelineArgs)
	if err != nil {
		return nil, err
	}

	return &pipelineRunner, err
}

func getArgs() map[string]string {
	argsWithoutProg := os.Args[1:]
	args := map[string]string{}
	for _, arg := range argsWithoutProg {
		keyValue := strings.Split(arg, "=")
		key := keyValue[0]
		value := keyValue[1]
		args[key] = value
	}
	return args
}
