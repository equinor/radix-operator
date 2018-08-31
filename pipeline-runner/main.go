package main

import (
	"os"
	"strings"

	logger "github.com/Sirupsen/logrus"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	pipe "github.com/statoil/radix-operator/pipeline-runner/onpush"
)

func main() {
	args := getArgs()
	appName := args["APP_NAME"]
	branch := args["BRANCH"]

	if appName == "" {
		appName = "radix-static-html"
	}
	if branch == "" {
		branch = "master"
	}

	client, radixClient := getKubernetesClient()
	pushHandler := pipe.Init(client, radixClient)
	pushHandler.Run(appName, branch)
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
