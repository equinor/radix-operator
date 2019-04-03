package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	pipe "github.com/equinor/radix-operator/pipeline-runner/pipelines"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
)

var (
	pipelineDate     string
	pipelineCommitid string
	pipelineBranch   string
)

func init() {
	if pipelineCommitid == "" {
		pipelineCommitid = "no commitid"
	}

	if pipelineBranch == "" {
		pipelineBranch = "no branch"
	}

	if pipelineDate == "" {
		pipelineDate = "(Mon YYYY)"
	}
}

// Requirements to run, pipeline must have:
// - access to read RR of the app mention in "RADIX_FILE_NAME"
// - access to create Jobs in "app" namespace it runs under
// - access to create RD in all namespaces
// - access to create new namespaces
// - a secret git-ssh-keys containing deployment key to git repo provided in RR
// - a secret radix-docker with credentials to access our private ACR
func main() {
	pipelineInfo, pushHandler := prepareToRunPipeline()

	err := pushHandler.Run(pipelineInfo)
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

// runs os.Exit(1) if error
func prepareToRunPipeline() (model.PipelineInfo, pipe.RadixOnPushHandler) {
	args := getArgs()
	branch := args["BRANCH"]
	commitID := args["COMMIT_ID"]
	fileName := args["RADIX_FILE_NAME"]
	imageTag := args["IMAGE_TAG"]
	jobName := args["JOB_NAME"]
	useCache := args["USE_CACHE"]

	log.Infof("Starting Radix Pipeline from commit %s on branch %s built %s", pipelineCommitid, pipelineBranch, pipelineDate)

	if branch == "" {
		branch = "dev"
	}
	if fileName == "" {
		fileName, _ = filepath.Abs("./pipelines/testdata/radixconfig.yaml")
	}
	if imageTag == "" {
		imageTag = "latest"
	}
	if useCache == "" {
		useCache = "true"
	}

	client, radixClient, prometheusOperatorClient := utils.GetKubernetesClient()
	pushHandler, err := pipe.Init(client, radixClient, prometheusOperatorClient)
	if err != nil {
		os.Exit(1)
	}

	radixApplication, err := loadConfigFromFile(fileName)
	if err != nil {
		os.Exit(1)
	}

	radixRegistration, targetEnvironments, err := pushHandler.Prepare(radixApplication, branch)
	if err != nil {
		os.Exit(1)
	}

	pipelineInfo := model.PipelineInfo{
		RadixRegistration:  radixRegistration,
		RadixApplication:   radixApplication,
		TargetEnvironments: targetEnvironments,
		JobName:            jobName,
		Branch:             branch,
		CommitID:           commitID,
		ImageTag:           imageTag,
		UseCache:           useCache,
	}

	return pipelineInfo, pushHandler
}

// LoadConfigFromFile loads radix config from appFileName
func loadConfigFromFile(appFileName string) (*v1.RadixApplication, error) {
	radixApplication, err := utils.GetRadixApplication(appFileName)
	if err != nil {
		log.Errorf("Failed to get ra from file (%s) for app Error: %v", appFileName, err)
		return nil, err
	}

	return radixApplication, nil
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
