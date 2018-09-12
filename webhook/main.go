package main

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/spf13/pflag"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"github.com/statoil/radix-operator/webhook/handler"
	"k8s.io/client-go/kubernetes"

	// Force loading of needed authentication library
	"net/http"
	"os"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	fs := initializeFlagSet()

	var (
		kubeconfig            = fs.String("kubeconfig", defaultKubeConfig(), "Absolute path to the kubeconfig file")
		port                  = fs.StringP("port", "p", defaultPort(), "The port for which we listen to events on")
		pipelineHandlerConfig handler.Config
	)

	fs.StringVar(&pipelineHandlerConfig.DockerRegistryPath, "docker-registry", defaultDockerRegistryPath(), "Private docker registry path")
	fs.StringVar(&pipelineHandlerConfig.WorkerImage, "worker-image", defaultWorkerImage(), "Kubernetes worker image")
	fs.StringVar(&pipelineHandlerConfig.RadixConfigBranch, "radix-config-branch", defaultConfigBranch(), "Branch name to pull radix config from")
	parseFlagsFromArgs(fs)

	client, radixClient := getKubernetesClient(*kubeconfig)
	logrus.Infof("Listen for incoming events on port %s", *port)

	err := http.ListenAndServe(fmt.Sprintf(":%s", *port), WebhookLog(client, radixClient, &pipelineHandlerConfig))
	if err != nil {
		logrus.Fatalf("Unable to start serving: %v", err)
	}
}

func initializeFlagSet() *pflag.FlagSet {
	// Flag domain.
	fs := pflag.NewFlagSet("default", pflag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "DESCRIPTION\n")
		fmt.Fprintf(os.Stderr, "  radix webhook.\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "FLAGS\n")
		fs.PrintDefaults()
	}
	return fs
}

func parseFlagsFromArgs(fs *pflag.FlagSet) {
	err := fs.Parse(os.Args[1:])
	switch {
	case err == pflag.ErrHelp:
		os.Exit(0)
	case err != nil:
		fmt.Fprintf(os.Stderr, "Error: %s\n\n", err.Error())
		fs.Usage()
		os.Exit(2)
	}
}

func WebhookLog(kubeclient kubernetes.Interface, radixclient radixclient.Interface, config *handler.Config) http.Handler {
	var p handler.WebhookListener

	p = handler.NewPipelineTrigger(kubeclient, config)
	wh := handler.NewWebHookHandler(radixclient)
	return wh.HandleWebhookEvents(p)
}

func getKubernetesClient(kubeConfigPath string) (kubernetes.Interface, radixclient.Interface) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			logrus.Fatalf("getClusterConfig InClusterConfig: %v", err)
		}
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		logrus.Fatalf("getClusterConfig k8s client: %v", err)
	}

	radixClient, err := radixclient.NewForConfig(config)
	if err != nil {
		logrus.Fatalf("getClusterConfig radix client: %v", err)
	}

	return client, radixClient
}

func defaultKubeConfig() string {
	return os.Getenv("HOME") + "/.kube/config"
}

func defaultConfigBranch() string {
	return "master"
}

func defaultDockerRegistryPath() string {
	return "radixdev.azurecr.io"
}

func defaultWorkerImage() string {
	return "radix-pipeline"
}

func defaultPort() string {
	return "3001"
}
