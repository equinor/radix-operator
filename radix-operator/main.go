package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	errorUtils "github.com/equinor/radix-common/utils/errors"
	apiconfig "github.com/equinor/radix-operator/pkg/apis/config"
	dnsaliasconfig "github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	deploymentAPI "github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixinformers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/equinor/radix-operator/radix-operator/alert"
	"github.com/equinor/radix-operator/radix-operator/application"
	"github.com/equinor/radix-operator/radix-operator/batch"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/equinor/radix-operator/radix-operator/config"
	"github.com/equinor/radix-operator/radix-operator/deployment"
	"github.com/equinor/radix-operator/radix-operator/dnsalias"
	"github.com/equinor/radix-operator/radix-operator/environment"
	"github.com/equinor/radix-operator/radix-operator/job"
	"github.com/equinor/radix-operator/radix-operator/registration"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/flowcontrol"
	secretproviderclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
)

const (
	resyncPeriod            = 0
	ingressConfigurationMap = "radix-operator-ingress-configmap"
)

var logger *log.Entry

func main() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": "main"})
	cfg := config.NewConfig()
	setLogLevel(cfg.LogLevel)

	registrationControllerThreads, applicationControllerThreads, environmentControllerThreads, deploymentControllerThreads, jobControllerThreads, alertControllerThreads, kubeClientRateLimitBurst, kubeClientRateLimitQPS, err := getInitParams()
	if err != nil {
		panic(err)
	}
	rateLimitConfig := utils.WithKubernetesClientRateLimiter(flowcontrol.NewTokenBucketRateLimiter(kubeClientRateLimitQPS, kubeClientRateLimitBurst))
	client, radixClient, prometheusOperatorClient, secretProviderClient := utils.GetKubernetesClient(rateLimitConfig)

	activeClusterNameEnvVar := os.Getenv(defaults.ActiveClusternameEnvironmentVariable)
	logger.Printf("Active cluster name: %v", activeClusterNameEnvVar)

	stop := make(chan struct{})
	defer close(stop)

	go startMetricsServer(stop)

	eventRecorder := common.NewEventRecorder("Radix controller", client.CoreV1().Events(""), logger)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, resyncPeriod)
	radixInformerFactory := radixinformers.NewSharedInformerFactory(radixClient, resyncPeriod)

	startController(createRegistrationController(client, radixClient, kubeInformerFactory, radixInformerFactory, eventRecorder, secretProviderClient), registrationControllerThreads, stop)
	startController(createApplicationController(client, radixClient, kubeInformerFactory, radixInformerFactory, eventRecorder, secretProviderClient, cfg.DNSConfig), applicationControllerThreads, stop)
	startController(createEnvironmentController(client, radixClient, kubeInformerFactory, radixInformerFactory, eventRecorder, secretProviderClient), environmentControllerThreads, stop)
	startController(createDeploymentController(client, radixClient, prometheusOperatorClient, kubeInformerFactory, radixInformerFactory, eventRecorder, secretProviderClient), deploymentControllerThreads, stop)
	startController(createJobController(client, radixClient, kubeInformerFactory, radixInformerFactory, eventRecorder, secretProviderClient, cfg), jobControllerThreads, stop)
	startController(createAlertController(client, radixClient, prometheusOperatorClient, kubeInformerFactory, radixInformerFactory, eventRecorder, secretProviderClient), alertControllerThreads, stop)
	startController(createBatchController(client, radixClient, kubeInformerFactory, radixInformerFactory, eventRecorder, secretProviderClient), 1, stop)
	startController(createDNSAliasesController(client, radixClient, kubeInformerFactory, radixInformerFactory, eventRecorder, secretProviderClient, cfg.DNSConfig), environmentControllerThreads, stop)

	// Start informers when all controllers are running
	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)

	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, syscall.SIGTERM)
	signal.Notify(sigTerm, syscall.SIGINT)
	<-sigTerm
}

func startController(controller *common.Controller, threadiness int, stop <-chan struct{}) {
	go func() {
		if err := controller.Run(threadiness, stop); err != nil {
			logger.Fatalf("Error running controller: %s", err.Error())
		}
	}()
}

func getInitParams() (int, int, int, int, int, int, int, float32, error) {
	registrationControllerThreads, regErr := defaults.GetRegistrationControllerThreads()
	applicationControllerThreads, appErr := defaults.GetApplicationControllerThreads()
	environmentControllerThreads, envErr := defaults.GetEnvironmentControllerThreads()
	deploymentControllerThreads, depErr := defaults.GetDeploymentControllerThreads()
	jobControllerThreads, jobErr := defaults.GetJobControllerThreads()
	alertControllerThreads, aleErr := defaults.GetAlertControllerThreads()
	kubeClientRateLimitBurst, burstErr := defaults.GetKubeClientRateLimitBurst()
	kubeClientRateLimitQPS, qpsErr := defaults.GetKubeClientRateLimitQps()

	errCat := errorUtils.Concat([]error{regErr, appErr, envErr, depErr, jobErr, aleErr, burstErr, qpsErr})
	return registrationControllerThreads, applicationControllerThreads, environmentControllerThreads, deploymentControllerThreads, jobControllerThreads, alertControllerThreads, kubeClientRateLimitBurst, kubeClientRateLimitQPS, errCat
}

func createRegistrationController(client kubernetes.Interface, radixClient radixclient.Interface, kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory, recorder record.EventRecorder, secretProviderClient secretproviderclient.Interface) *common.Controller {
	kubeUtil, _ := kube.NewWithListers(
		client,
		radixClient,
		secretProviderClient,
		kubeInformerFactory,
		radixInformerFactory,
	)

	handler := registration.NewHandler(
		client,
		kubeUtil,
		radixClient,
		func(syncedOk bool) {}, // Not interested in getting notifications of synced
	)

	const waitForChildrenToSync = true
	return registration.NewController(
		client,
		radixClient,
		&handler,
		kubeInformerFactory,
		radixInformerFactory,
		waitForChildrenToSync,
		recorder)
}

func createApplicationController(client kubernetes.Interface, radixClient radixclient.Interface, kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory, recorder record.EventRecorder, secretProviderClient secretproviderclient.Interface, dnsConfig *dnsaliasconfig.DNSConfig) *common.Controller {
	kubeUtil, _ := kube.NewWithListers(
		client,
		radixClient,
		secretProviderClient,
		kubeInformerFactory,
		radixInformerFactory,
	)

	handler := application.NewHandler(client,
		kubeUtil,
		radixClient,
		dnsConfig,
		func(syncedOk bool) {}, // Not interested in getting notifications of synced
	)

	const waitForChildrenToSync = true
	return application.NewController(
		client,
		radixClient,
		&handler,
		kubeInformerFactory,
		radixInformerFactory,
		waitForChildrenToSync,
		recorder)
}

func createEnvironmentController(client kubernetes.Interface, radixClient radixclient.Interface, kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory, recorder record.EventRecorder, secretProviderClient secretproviderclient.Interface) *common.Controller {
	kubeUtil, _ := kube.NewWithListers(
		client,
		radixClient,
		secretProviderClient,
		kubeInformerFactory,
		radixInformerFactory,
	)

	handler := environment.NewHandler(
		client,
		kubeUtil,
		radixClient,
		func(syncedOk bool) {}, // Not interested in getting notifications of synced
	)

	const waitForChildrenToSync = true
	return environment.NewController(
		client,
		radixClient,
		&handler,
		kubeInformerFactory,
		radixInformerFactory,
		waitForChildrenToSync,
		recorder)
}

func createDNSAliasesController(client kubernetes.Interface, radixClient radixclient.Interface, kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory, recorder record.EventRecorder, secretProviderClient secretproviderclient.Interface, dnsConfig *dnsaliasconfig.DNSConfig) *common.Controller {
	kubeUtil, _ := kube.NewWithListers(
		client,
		radixClient,
		secretProviderClient,
		kubeInformerFactory,
		radixInformerFactory,
	)

	handler := dnsalias.NewHandler(
		client,
		kubeUtil,
		radixClient,
		dnsConfig,
		func(syncedOk bool) {}, // Not interested in getting notifications of synced
	)

	const waitForChildrenToSync = true
	return dnsalias.NewController(
		client,
		radixClient,
		handler,
		kubeInformerFactory,
		radixInformerFactory,
		waitForChildrenToSync,
		recorder)
}

func createDeploymentController(client kubernetes.Interface, radixClient radixclient.Interface, prometheusOperatorClient monitoring.Interface, kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory, recorder record.EventRecorder, secretProviderClient secretproviderclient.Interface) *common.Controller {
	kubeUtil, _ := kube.NewWithListers(
		client,
		radixClient,
		secretProviderClient,
		kubeInformerFactory,
		radixInformerFactory,
	)

	oauthDefaultConfig := defaults.NewOAuth2Config(
		defaults.WithOAuth2Defaults(),
		defaults.WithOIDCIssuerURL(os.Getenv(defaults.RadixOAuthProxyDefaultOIDCIssuerURLEnvironmentVariable)),
	)
	ingressConfiguration, err := loadIngressConfigFromMap(kubeUtil)
	if err != nil {
		panic(fmt.Errorf("failed to load ingress configuration: %v", err))
	}

	oauth2DockerImage := os.Getenv(defaults.RadixOAuthProxyImageEnvironmentVariable)
	if oauth2DockerImage == "" {
		panic(fmt.Errorf("failed to read OAuth2 Docker image from environment variable %s", defaults.RadixOAuthProxyImageEnvironmentVariable))
	}
	handler := deployment.NewHandler(client,
		kubeUtil,
		radixClient,
		prometheusOperatorClient,
		deployment.WithTenantIdFromEnvVar(defaults.OperatorTenantIdEnvironmentVariable),
		deployment.WithKubernetesApiPortFromEnvVar(defaults.KubernetesApiPortEnvironmentVariable),
		deployment.WithDeploymentHistoryLimitFromEnvVar(defaults.DeploymentsHistoryLimitEnvironmentVariable),
		deployment.WithOAuth2DefaultConfig(oauthDefaultConfig),
		deployment.WithIngressConfiguration(ingressConfiguration),
		deployment.WithOAuth2ProxyDockerImage(oauth2DockerImage),
	)

	const waitForChildrenToSync = true
	return deployment.NewController(
		client,
		radixClient,
		handler,
		kubeInformerFactory,
		radixInformerFactory,
		waitForChildrenToSync,
		recorder)
}

func createJobController(client kubernetes.Interface, radixClient radixclient.Interface, kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory, recorder record.EventRecorder, secretProviderClient secretproviderclient.Interface, config *apiconfig.Config) *common.Controller {
	kubeUtil, _ := kube.NewWithListers(
		client,
		radixClient,
		secretProviderClient,
		kubeInformerFactory,
		radixInformerFactory,
	)

	handler := job.NewHandler(client, kubeUtil, radixClient, config, func(syncedOk bool) {}) // Not interested in getting notifications of synced

	const waitForChildrenToSync = true
	return job.NewController(client, radixClient, &handler, kubeInformerFactory, radixInformerFactory, waitForChildrenToSync, recorder)
}

func createAlertController(client kubernetes.Interface, radixClient radixclient.Interface, prometheusOperatorClient monitoring.Interface, kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory, recorder record.EventRecorder, secretProviderClient secretproviderclient.Interface) *common.Controller {
	kubeUtil, _ := kube.NewWithListers(
		client,
		radixClient,
		secretProviderClient,
		kubeInformerFactory,
		radixInformerFactory,
	)

	handler := alert.NewHandler(client,
		kubeUtil,
		radixClient,
		prometheusOperatorClient,
	)

	const waitForChildrenToSync = true
	return alert.NewController(
		client,
		radixClient,
		handler,
		kubeInformerFactory,
		radixInformerFactory,
		waitForChildrenToSync,
		recorder)
}

func createBatchController(client kubernetes.Interface, radixClient radixclient.Interface, kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory, recorder record.EventRecorder, secretProviderClient secretproviderclient.Interface) *common.Controller {
	kubeUtil, _ := kube.NewWithListers(
		client,
		radixClient,
		secretProviderClient,
		kubeInformerFactory,
		radixInformerFactory,
	)

	handler := batch.NewHandler(
		client,
		kubeUtil,
		radixClient,
	)

	const waitForChildrenToSync = true
	return batch.NewController(
		client,
		radixClient,
		handler,
		kubeInformerFactory,
		radixInformerFactory,
		waitForChildrenToSync,
		recorder)
}

func loadIngressConfigFromMap(kubeutil *kube.Kube) (deploymentAPI.IngressConfiguration, error) {
	ingressConfig := deploymentAPI.IngressConfiguration{}
	configMap, err := kubeutil.GetConfigMap(metav1.NamespaceDefault, ingressConfigurationMap)
	if err != nil {
		return ingressConfig, err
	}

	err = yaml.Unmarshal([]byte(configMap.Data["ingressConfiguration"]), &ingressConfig)
	if err != nil {
		return ingressConfig, err
	}
	return ingressConfig, nil
}

func startMetricsServer(stop <-chan struct{}) {
	srv := &http.Server{Addr: ":9000"}
	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/healthz", http.HandlerFunc(Healthz))
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("MetricServer: ListenAndServe() error: %s", err)
		}
	}()
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Errorf("shutdown metrics server failed: %v", err)
	}
}

// HealthStatus Represents the data of the endpoint
type HealthStatus struct {
	Status int
}

// Healthz The health endpoint
func Healthz(writer http.ResponseWriter, _ *http.Request) {
	health := HealthStatus{
		Status: http.StatusOK,
	}

	response, err := json.Marshal(health)

	if err != nil {
		http.Error(writer, "Error while retrieving HealthStatus", http.StatusInternalServerError)
		logger.Errorf("Could not serialize HealthStatus: %v", err)
		return
	}

	_, _ = fmt.Fprintf(writer, "%s", response)
}

func setLogLevel(logLevel apiconfig.LogLevel) {
	switch logLevel {
	case apiconfig.LogLevelDebug:
		log.SetLevel(log.DebugLevel)
	case apiconfig.LogLevelWarning:
		log.SetLevel(log.WarnLevel)
	case apiconfig.LogLevelError:
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}
}
