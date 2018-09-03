package main

import (
	"os"
	"path/filepath"
	"strings"

	logger "github.com/Sirupsen/logrus"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	pipe "github.com/statoil/radix-operator/pipeline-runner/onpush"
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
		fileName, _ = filepath.Abs("./onpush/testdata/radixconfig.yaml")
	}
	if imageTag == "" {
		imageTag = "latest"
	}

	client, radixClient := getKubernetesClient()
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

func getKubernetesClient() (kubernetes.Interface, radixclient.Interface) {
	kubeConfigPath := os.Getenv("HOME") + "/.kube/config"
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			logger.Fatalf("getClusterConfig InClusterConfig: %v", err)
		}
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatalf("getClusterConfig k8s client: %v", err)
	}

	radixClient, err := radixclient.NewForConfig(config)
	if err != nil {
		logger.Fatalf("getClusterConfig radix client: %v", err)
	}

	logger.Printf("Successfully constructed k8s client to API server %v", config.Host)
	return client, radixClient
}
