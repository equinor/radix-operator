package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/statoil/radix-operator/pkg/apis/kube"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	log "github.com/Sirupsen/logrus"
	pipe "github.com/statoil/radix-operator/pipeline-runner/pipelines"
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
	args := getArgs()
	branch := args["BRANCH"]
	commitID := args["COMMIT_ID"]
	fileName := args["RADIX_FILE_NAME"]
	imageTag := args["IMAGE_TAG"]
	jobName := args["JOB_NAME"]

	log.Infof("Starting Radix Pipeline from commit %s on branch %s built %s", pipelineCommitid, pipelineBranch, pipelineDate)

	if branch == "" {
		branch = "master"
	}
	if fileName == "" {
		fileName, _ = filepath.Abs("./pipelines/testdata/radixconfig.yaml")
	}
	if imageTag == "" {
		imageTag = "latest2"
	}

	client, radixClient := kube.GetKubernetesClient()
	pushHandler, err := pipe.Init(client, radixClient)
	if err != nil {
		os.Exit(1)
	}

	err = pushHandler.Run(jobName, branch, commitID, imageTag, fileName)
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
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
