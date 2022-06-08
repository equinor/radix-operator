package utils

import (
	"net/http"
	"os"

	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/flowcontrol"
	secretProviderClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
)

var (
	nrRequests = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "radix_operator_request",
		Help: "The total number of http requests done from radix operator",
	}, []string{"code", "method"})
)

type KubernetesClientConfigOption func(*rest.Config)

func WithKubernetesClientRateLimiter(rateLimiter flowcontrol.RateLimiter) KubernetesClientConfigOption {
	return func(c *rest.Config) {
		c.RateLimiter = rateLimiter
	}
}

// GetKubernetesClient Gets clients to talk to the API
func GetKubernetesClient(configOptions ...KubernetesClientConfigOption) (kubernetes.Interface, radixclient.Interface, monitoring.Interface, secretProviderClient.Interface) {
	kubeConfigPath := os.Getenv("HOME") + "/.kube/config"
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)

	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Fatalf("getClusterConfig InClusterConfig: %v", err)
		}
	}
	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return promhttp.InstrumentRoundTripperDuration(nrRequests, rt)
	}

	for _, o := range configOptions {
		o(config)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("getClusterConfig k8s client: %v", err)
	}

	radixClient, err := radixclient.NewForConfig(config)
	if err != nil {
		log.Fatalf("getClusterConfig radix client: %v", err)
	}

	prometheusOperatorClient, err := monitoring.NewForConfig(config)
	if err != nil {
		log.Fatalf("getClusterConfig prometheus-operator client: %v", err)
	}

	secretProviderClient, err := secretProviderClient.NewForConfig(config)
	if err != nil {
		log.Fatalf("secretProvider secret provider client client: %v", err)
	}

	log.Printf("Successfully constructed k8s client to API server %v", config.Host)
	return client, radixClient, prometheusOperatorClient, secretProviderClient
}
