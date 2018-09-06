package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/statoil/radix-operator/pkg/apis/kube"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	pipe "github.com/statoil/radix-operator/pipeline-runner/pipelines"
)

// should we have different pipeline types? if yes, should each be a small go script?
// should run in app namespace, where users has access to read pods, jobs, logs (not secrets)
// pipeline runner should be registered as a job running in app namespace,
// pointing to pipeline-runner image, with labels to identify runned pipelines
func main() {
	args := getArgs()
	branch := args["BRANCH"]
	fileName := args["RADIX_FILE_NAME"]
	imageTag := args["IMAGE_TAG"]

	if branch == "" {
		branch = "master"
	}
	if fileName == "" {
		fileName, _ = filepath.Abs("./pipelines/testdata/radixconfig.yaml")
	}
	if imageTag == "" {
		imageTag = "latest"
	}

	client, radixClient := kube.GetKubernetesClient()
	pushHandler, err := pipe.Init(client, radixClient)
	if err != nil {
		os.Exit(1)
	}

	err = pushHandler.Run(branch, imageTag, fileName)
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
