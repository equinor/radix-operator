package utils

import (
	"context"
	"net/http"
	"os"
	"time"

	certclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
func GetKubernetesClient(ctx context.Context, configOptions ...KubernetesClientConfigOption) (kubernetes.Interface, radixclient.Interface, kedav2.Interface, monitoring.Interface, secretProviderClient.Interface, certclient.Interface) {
	pollTimeout, pollInterval := time.Minute, 15*time.Second
	kubeConfigPath := os.Getenv("HOME") + "/.kube/config"
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)

	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to read InClusterConfig")
		}
	}

	config.WarningHandler = rest.NoWarnings{}
	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return promhttp.InstrumentRoundTripperDuration(nrRequests, rt)
	}

	for _, o := range configOptions {
		o(config)
	}

	client, err := PollUntilRESTClientSuccessfulConnection(ctx, pollTimeout, pollInterval, func() (*kubernetes.Clientset, error) {
		return kubernetes.NewForConfig(config)
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Kubernetes client")
	}

	radixClient, err := PollUntilRESTClientSuccessfulConnection(ctx, pollTimeout, pollInterval, func() (*radixclient.Clientset, error) {
		return radixclient.NewForConfig(config)
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Radix client")
	}

	kedaClient, err := PollUntilRESTClientSuccessfulConnection(ctx, pollTimeout, pollInterval, func() (*kedav2.Clientset, error) {
		return kedav2.NewForConfig(config)
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Prometheus client")
	}

	prometheusOperatorClient, err := PollUntilRESTClientSuccessfulConnection(ctx, pollTimeout, pollInterval, func() (*monitoring.Clientset, error) {
		return monitoring.NewForConfig(config)
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Prometheus client")
	}

	secretProviderClient, err := PollUntilRESTClientSuccessfulConnection(ctx, pollTimeout, pollInterval, func() (*secretProviderClient.Clientset, error) {
		return secretProviderClient.NewForConfig(config)
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize SecretProvider client")
	}
	certClient, err := PollUntilRESTClientSuccessfulConnection(ctx, pollTimeout, pollInterval, func() (*certclient.Clientset, error) {
		return certclient.NewForConfig(config)
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize CertManager client")
	}

	log.Info().Msgf("Successfully constructed k8s client to API server %v", config.Host)
	return client, radixClient, kedaClient, prometheusOperatorClient, secretProviderClient, certClient
}
