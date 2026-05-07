package utils

import (
	"net/http"
	"os"

	certclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	radixmodels "github.com/equinor/radix-common/models"
	"github.com/equinor/radix-operator/api-server/api/metrics"
	"github.com/equinor/radix-operator/api-server/api/utils/logs"
	"github.com/equinor/radix-operator/api-server/api/utils/warningcollector"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	secretproviderclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
)

type RestClientConfigOption func(*restclient.Config)

func WithQPS(qps float32) RestClientConfigOption {
	return func(cfg *restclient.Config) {
		cfg.QPS = qps
	}
}

func WithBurst(burst int) RestClientConfigOption {
	return func(cfg *restclient.Config) {
		cfg.Burst = burst
	}
}

// KubeUtil Interface to be mocked in tests
type KubeUtil interface {
	GetUserKubernetesClient(string, radixmodels.Impersonation, ...RestClientConfigOption) (kubernetes.Interface, radixclient.Interface, kedav2.Interface, secretproviderclient.Interface, tektonclient.Interface, certclient.Interface)
	GetServerKubernetesClient(...RestClientConfigOption) (kubernetes.Interface, radixclient.Interface, kedav2.Interface, secretproviderclient.Interface, tektonclient.Interface, certclient.Interface)
}

type kubeUtil struct{}

var (
	nrRequests = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "radix_api_k8s_request_duration_seconds",
		Help:    "request duration done to k8s api in seconds bucket",
		Buckets: metrics.DefaultBuckets(),
	}, []string{"code", "method"})
)

// NewKubeUtil Constructor
func NewKubeUtil() KubeUtil {
	return &kubeUtil{}
}

// GetUserKubernetesClient Gets a kubernetes client using the bearer token from the radix api client
func (ku *kubeUtil) GetUserKubernetesClient(token string, impersonation radixmodels.Impersonation, options ...RestClientConfigOption) (kubernetes.Interface, radixclient.Interface, kedav2.Interface, secretproviderclient.Interface, tektonclient.Interface, certclient.Interface) {
	config := getUserClientConfig(token, impersonation, options)
	return getKubernetesClientFromConfig(config)
}

// GetServerKubernetesClient Gets a kubernetes client using the config of host or pod
func (ku *kubeUtil) GetServerKubernetesClient(options ...RestClientConfigOption) (kubernetes.Interface, radixclient.Interface, kedav2.Interface, secretproviderclient.Interface, tektonclient.Interface, certclient.Interface) {
	config := getServerClientConfig(options)
	return getKubernetesClientFromConfig(config)
}

func getUserClientConfig(token string, impersonation radixmodels.Impersonation, options []RestClientConfigOption) *restclient.Config {
	cfg := getServerClientConfig(options)

	kubeConfig := &restclient.Config{
		Host:        cfg.Host,
		BearerToken: token,
		TLSClientConfig: restclient.TLSClientConfig{
			Insecure: true,
		},
	}

	if impersonation.PerformImpersonation() {
		impersonationConfig := restclient.ImpersonationConfig{
			UserName: impersonation.User,
			Groups:   impersonation.Groups,
		}

		kubeConfig.Impersonate = impersonationConfig
	}
	kubeConfig.Wrap(logs.NewRoundtripLogger(func(e *zerolog.Event) {
		e.Str("client", "user")
	}))

	return addCommonConfigs(kubeConfig, options)
}

func getServerClientConfig(options []RestClientConfigOption) *restclient.Config {
	kubeConfigPath := os.Getenv("HOME") + "/.kube/config"
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		config, err = restclient.InClusterConfig()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create in cluster config")
		}
	}
	config.Wrap(logs.NewRoundtripLogger(func(e *zerolog.Event) {
		e.Str("client", "server")
	}))

	return addCommonConfigs(config, options)
}

func addCommonConfigs(config *restclient.Config, options []RestClientConfigOption) *restclient.Config {
	for _, opt := range options {
		opt(config)
	}
	config.Wrap(func(rt http.RoundTripper) http.RoundTripper {
		return promhttp.InstrumentRoundTripperDuration(nrRequests, rt)
	})

	config.WarningHandlerWithContext = warningcollector.NewKubernetesWarningHandler()

	return config
}

func getKubernetesClientFromConfig(config *restclient.Config) (kubernetes.Interface, radixclient.Interface, kedav2.Interface, secretproviderclient.Interface, tektonclient.Interface, certclient.Interface) {
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal().Err(err).Msg("getClusterConfig k8s client")
	}

	radixClient, err := radixclient.NewForConfig(config)
	if err != nil {
		log.Fatal().Err(err).Msg("getClusterConfig radix client")
	}

	kedaClient, err := kedav2.NewForConfig(config)
	if err != nil {
		log.Fatal().Err(err).Msg("getClusterConfig keda client")
	}

	secretProviderClient, err := secretproviderclient.NewForConfig(config)
	if err != nil {
		log.Fatal().Err(err).Msg("getClusterConfig secret provider client client")
	}

	tektonClient, err := tektonclient.NewForConfig(config)
	if err != nil {
		log.Fatal().Err(err).Msg("getClusterConfig Tekton client client")
	}

	certClient, err := certclient.NewForConfig(config)
	if err != nil {
		log.Fatal().Err(err).Msg("getClusterConfig Tekton client client")
	}
	return client, radixClient, kedaClient, secretProviderClient, tektonClient, certClient
}
