package commands

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"

	// Kube client doesn't support all auth providers by default.
	// this ensures we include all backends supported by the client.
	"k8s.io/client-go/kubernetes"
	// auth is a side-effect import for Client-Go
	// _ "k8s.io/client-go/plugin/pkg/client/auth"
	rxv1 "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	globalNamespace   string
	globalKubeConfig  string
	globalKubeContext string
	globalVerbose     bool
)

const mainUsage = `Interact with the Radix platform.

Rx is a tool to work with deployments and such.
`

func init() {
	f := Root.PersistentFlags()
	f.StringVarP(&globalNamespace, "namespace", "n", "default", "The Kubernetes namespace for Brigade")
	f.StringVar(&globalKubeConfig, "kubeconfig", "", "The path to a KUBECONFIG file, overrides $KUBECONFIG.")
	f.StringVar(&globalKubeContext, "kube-context", "", "The name of the kubeconfig context to use.")
	f.BoolVarP(&globalVerbose, "verbose", "v", false, "Turn on verbose output")
}

// Root is the root command
var Root = &cobra.Command{
	Use:   "rx",
	Short: "The Radix client",
	Long:  mainUsage,
}

// kubeClient returns a Kubernetes clientset.
func kubeClient() (*kubernetes.Clientset, error) {
	cfg, err := getKubeConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(cfg)
}

func radixClient() (*rxv1.Clientset, error) {
	cfg, err := getKubeConfig()
	if err != nil {
		log.Errorf("Failed to get k8s config: %v", err)
		return nil, err
	}
	return rxv1.NewForConfig(cfg)
}

// getKubeConfig returns a Kubernetes client config.
func getKubeConfig() (*rest.Config, error) {
	kubeConfigPath := os.Getenv("HOME") + "/.kube/config"
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Fatalf("getClusterConfig InClusterConfig: %v", err)
		}
	}
	return config, nil
	// rules := clientcmd.NewDefaultClientConfigLoadingRules()
	// rules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	// rules.ExplicitPath = globalKubeConfig

	// overrides := &clientcmd.ConfigOverrides{
	// 	ClusterDefaults: clientcmd.ClusterDefaults,
	// 	CurrentContext:  globalKubeContext,
	// }

	// return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
}
