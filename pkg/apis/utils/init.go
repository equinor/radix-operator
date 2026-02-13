package utils

import (
	"context"
	"net/http"
	"os"

	certclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	httputils "github.com/equinor/radix-operator/pkg/apis/utils/http"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
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

func WithKubernetesWarningHandler(handler rest.WarningHandler) KubernetesClientConfigOption {
	return func(c *rest.Config) {
		c.WarningHandler = handler
	}
}

type ZerologWarningHandlerAdapter func() *zerolog.Event

func (zl ZerologWarningHandlerAdapter) HandleWarningHeader(_ int, _ string, text string) {
	zl().Msg(text)
}

// GetKubernetesClient Gets clients to talk to the API
func GetKubernetesClient(configOptions ...KubernetesClientConfigOption) (kubernetes.Interface, radixclient.Interface, kedav2.Interface, secretProviderClient.Interface, certclient.Interface, tektonclient.Interface) {
	kubeConfigPath := os.Getenv("HOME") + "/.kube/config"
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to read InClusterConfig")
		}
	}
	config.WarningHandler = rest.NoWarnings{}
	config.Wrap(PrometheusMetrics)
	config.Wrap(httputils.LogRequests)

	for _, o := range configOptions {
		o(config)
	}

	k8sclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Kubernetes client")
	}

	radixClient, err := radixclient.NewForConfig(config)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Radix client")
	}

	kedaClient, err := kedav2.NewForConfig(config)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize KEDA client")
	}

	secretProviderClient, err := secretProviderClient.NewForConfig(config)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize SecretProvider client")
	}
	certClient, err := certclient.NewForConfig(config)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize CertManager client")
	}

	tektonClient, err := tektonclient.NewForConfig(config)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Tekton client")
	}
	log.Info().Msgf("Successfully constructed k8s client to API server %v", config.Host)
	return k8sclient, radixClient, kedaClient, secretProviderClient, certClient, tektonClient
}

func PrometheusMetrics(rt http.RoundTripper) http.RoundTripper {
	return promhttp.InstrumentRoundTripperDuration(nrRequests, rt)
}
