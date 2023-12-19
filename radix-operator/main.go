package main

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	apiconfig "github.com/equinor/radix-operator/pkg/apis/config"
	dnsaliasconfig "github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
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
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/flowcontrol"
)

const (
	resyncPeriod            = 0
	ingressConfigurationMap = "radix-operator-ingress-configmap"
)

var logger *log.Entry

func main() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": "main"})
	cfg := config.NewConfig()
	log.SetLevel(cfg.LogLevel)

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
	kubeUtil, _ := kube.NewWithListers(
		client,
		radixClient,
		secretProviderClient,
		kubeInformerFactory,
		radixInformerFactory,
	)
	oauthDefaultConfig := getOAuthDefaultConfig()
	ingressConfiguration, err := loadIngressConfigFromMap(kubeUtil)
	if err != nil {
		panic(fmt.Errorf("failed to load ingress configuration: %v", err))
	}

	startController(createRegistrationController(kubeUtil, kubeInformerFactory, radixInformerFactory, eventRecorder), registrationControllerThreads, stop)
	startController(createApplicationController(kubeUtil, kubeInformerFactory, radixInformerFactory, eventRecorder, cfg.DNSConfig), applicationControllerThreads, stop)
	startController(createEnvironmentController(kubeUtil, kubeInformerFactory, radixInformerFactory, eventRecorder), environmentControllerThreads, stop)
	startController(createDeploymentController(kubeUtil, prometheusOperatorClient, kubeInformerFactory, radixInformerFactory, eventRecorder, oauthDefaultConfig, ingressConfiguration, cfg), deploymentControllerThreads, stop)
	startController(createJobController(kubeUtil, kubeInformerFactory, radixInformerFactory, eventRecorder, cfg), jobControllerThreads, stop)
	startController(createAlertController(kubeUtil, prometheusOperatorClient, kubeInformerFactory, radixInformerFactory, eventRecorder), alertControllerThreads, stop)
	startController(createBatchController(kubeUtil, kubeInformerFactory, radixInformerFactory, eventRecorder), 1, stop)
	startController(createDNSAliasesController(kubeUtil, kubeInformerFactory, radixInformerFactory, eventRecorder, oauthDefaultConfig, ingressConfiguration, cfg.DNSConfig), environmentControllerThreads, stop)

	// Start informers when all controllers are running
	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)

	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, syscall.SIGTERM)
	signal.Notify(sigTerm, syscall.SIGINT)
	<-sigTerm
}

func getOAuthDefaultConfig() defaults.OAuth2Config {
	return defaults.NewOAuth2Config(
		defaults.WithOAuth2Defaults(),
		defaults.WithOIDCIssuerURL(os.Getenv(defaults.RadixOAuthProxyDefaultOIDCIssuerURLEnvironmentVariable)),
	)
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

	errCat := stderrors.Join(regErr, appErr, envErr, depErr, jobErr, aleErr, burstErr, qpsErr)
	return registrationControllerThreads, applicationControllerThreads, environmentControllerThreads, deploymentControllerThreads, jobControllerThreads, alertControllerThreads, kubeClientRateLimitBurst, kubeClientRateLimitQPS, errCat
}

func createRegistrationController(kubeUtil *kube.Kube, kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory, recorder record.EventRecorder) *common.Controller {
	handler := registration.NewHandler(
		kubeUtil.KubeClient(),
		kubeUtil,
		kubeUtil.RadixClient(),
		func(syncedOk bool) {}, // Not interested in getting notifications of synced
	)

	return registration.NewController(
		kubeUtil.KubeClient(),
		kubeUtil.RadixClient(),
		&handler,
		kubeInformerFactory,
		radixInformerFactory,
		true,
		recorder)
}

func createApplicationController(kubeUtil *kube.Kube, kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory, recorder record.EventRecorder, dnsConfig *dnsaliasconfig.DNSConfig) *common.Controller {
	handler := application.NewHandler(
		kubeUtil.KubeClient(),
		kubeUtil,
		kubeUtil.RadixClient(),
		dnsConfig,
		func(syncedOk bool) {}, // Not interested in getting notifications of synced
	)

	return application.NewController(
		kubeUtil.KubeClient(),
		kubeUtil.RadixClient(),
		&handler,
		kubeInformerFactory,
		radixInformerFactory,
		true,
		recorder)
}

func createEnvironmentController(kubeUtil *kube.Kube, kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory, recorder record.EventRecorder) *common.Controller {
	handler := environment.NewHandler(
		kubeUtil.KubeClient(),
		kubeUtil,
		kubeUtil.RadixClient(),
		func(syncedOk bool) {}, // Not interested in getting notifications of synced
	)

	return environment.NewController(
		kubeUtil.KubeClient(),
		kubeUtil.RadixClient(),
		&handler,
		kubeInformerFactory,
		radixInformerFactory,
		true,
		recorder)
}

func createDNSAliasesController(kubeUtil *kube.Kube,
	kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory,
	recorder record.EventRecorder, oauthDefaultConfig defaults.OAuth2Config, ingressConfiguration ingress.IngressConfiguration,
	dnsConfig *dnsaliasconfig.DNSConfig) *common.Controller {

	handler := dnsalias.NewHandler(
		kubeUtil.KubeClient(),
		kubeUtil,
		kubeUtil.RadixClient(),
		dnsConfig,
		func(syncedOk bool) {}, // Not interested in getting notifications of synced
		dnsalias.WithIngressConfiguration(ingressConfiguration),
		dnsalias.WithOAuth2DefaultConfig(oauthDefaultConfig),
	)

	return dnsalias.NewController(
		kubeUtil.KubeClient(),
		kubeUtil.RadixClient(),
		handler,
		kubeInformerFactory,
		radixInformerFactory,
		true,
		recorder)
}

func createDeploymentController(kubeUtil *kube.Kube, prometheusOperatorClient monitoring.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory,
	recorder record.EventRecorder, oauthDefaultConfig defaults.OAuth2Config, ingressConfiguration ingress.IngressConfiguration, config *apiconfig.Config) *common.Controller {

	oauth2DockerImage := os.Getenv(defaults.RadixOAuthProxyImageEnvironmentVariable)
	if oauth2DockerImage == "" {
		panic(fmt.Errorf("failed to read OAuth2 Docker image from environment variable %s", defaults.RadixOAuthProxyImageEnvironmentVariable))
	}
	handler := deployment.NewHandler(
		kubeUtil.KubeClient(),
		kubeUtil,
		kubeUtil.RadixClient(),
		prometheusOperatorClient,
		config,
		deployment.WithTenantIdFromEnvVar(defaults.OperatorTenantIdEnvironmentVariable),
		deployment.WithKubernetesApiPortFromEnvVar(defaults.KubernetesApiPortEnvironmentVariable),
		deployment.WithDeploymentHistoryLimitFromEnvVar(defaults.DeploymentsHistoryLimitEnvironmentVariable),
		deployment.WithOAuth2DefaultConfig(oauthDefaultConfig),
		deployment.WithIngressConfiguration(ingressConfiguration),
		deployment.WithOAuth2ProxyDockerImage(oauth2DockerImage),
	)

	return deployment.NewController(
		kubeUtil.KubeClient(),
		kubeUtil.RadixClient(),
		handler,
		kubeInformerFactory,
		radixInformerFactory,
		true,
		recorder)
}

func createJobController(kubeUtil *kube.Kube, kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory, recorder record.EventRecorder, config *apiconfig.Config) *common.Controller {
	handler := job.NewHandler(
		kubeUtil.KubeClient(),
		kubeUtil,
		kubeUtil.RadixClient(),
		config,
		func(syncedOk bool) {}) // Not interested in getting notifications of synced

	return job.NewController(
		kubeUtil.KubeClient(),
		kubeUtil.RadixClient(),
		&handler, kubeInformerFactory, radixInformerFactory, true, recorder)
}

func createAlertController(kubeUtil *kube.Kube, prometheusOperatorClient monitoring.Interface, kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory, recorder record.EventRecorder) *common.Controller {
	handler := alert.NewHandler(
		kubeUtil.KubeClient(),
		kubeUtil,
		kubeUtil.RadixClient(),
		prometheusOperatorClient,
	)

	return alert.NewController(
		kubeUtil.KubeClient(),
		kubeUtil.RadixClient(),
		handler,
		kubeInformerFactory,
		radixInformerFactory,
		true,
		recorder)
}

func createBatchController(kubeUtil *kube.Kube, kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory, recorder record.EventRecorder) *common.Controller {
	handler := batch.NewHandler(
		kubeUtil.KubeClient(),
		kubeUtil,
		kubeUtil.RadixClient(),
	)

	return batch.NewController(
		kubeUtil.KubeClient(),
		kubeUtil.RadixClient(),
		handler,
		kubeInformerFactory,
		radixInformerFactory,
		true,
		recorder)
}

func loadIngressConfigFromMap(kubeutil *kube.Kube) (ingress.IngressConfiguration, error) {
	ingressConfig := ingress.IngressConfiguration{}
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
