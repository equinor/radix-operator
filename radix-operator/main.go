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

	jobUtil "github.com/equinor/radix-operator/pkg/apis/job"

	errorUtils "github.com/equinor/radix-common/utils/errors"
	applicationAPI "github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	deploymentAPI "github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/equinor/radix-operator/radix-operator/alert"
	"github.com/equinor/radix-operator/radix-operator/application"
	"github.com/equinor/radix-operator/radix-operator/batch"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/equinor/radix-operator/radix-operator/config"
	"github.com/equinor/radix-operator/radix-operator/deployment"
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
	secretProviderClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
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

	activeclusternameEnvVar := os.Getenv(defaults.ActiveClusternameEnvironmentVariable)
	logger.Printf("Active cluster name: %v", activeclusternameEnvVar)

	stop := make(chan struct{})
	defer close(stop)

	go startMetricsServer(stop)

	eventRecorder := common.NewEventRecorder("Radix controller", client.CoreV1().Events(""), logger)

	go startRegistrationController(client, radixClient, eventRecorder, stop, secretProviderClient, registrationControllerThreads)
	go startApplicationController(client, radixClient, eventRecorder, stop, secretProviderClient, applicationControllerThreads)
	go startEnvironmentController(client, radixClient, eventRecorder, stop, secretProviderClient, environmentControllerThreads)
	go startDeploymentController(client, radixClient, prometheusOperatorClient, eventRecorder, stop, secretProviderClient, deploymentControllerThreads)
	go startJobController(client, radixClient, eventRecorder, stop, secretProviderClient, jobControllerThreads, cfg.PipelineJobConfig)
	go startAlertController(client, radixClient, prometheusOperatorClient, eventRecorder, stop, secretProviderClient, alertControllerThreads)
	go startBatchController(client, radixClient, eventRecorder, stop, secretProviderClient, 1)

	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, syscall.SIGTERM)
	signal.Notify(sigTerm, syscall.SIGINT)
	<-sigTerm
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

func startRegistrationController(client kubernetes.Interface, radixClient radixclient.Interface, recorder record.EventRecorder, stop <-chan struct{}, secretProviderClient secretProviderClient.Interface, threads int) {

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, resyncPeriod)
	radixInformerFactory := informers.NewSharedInformerFactory(radixClient, resyncPeriod)

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

		// Pass the default granter function to grant access to the service account token
		applicationAPI.GrantAppAdminAccessToMachineUserToken,
	)

	waitForChildrenToSync := true
	registrationController := registration.NewController(
		client,
		radixClient,
		&handler,
		kubeInformerFactory,
		radixInformerFactory,
		waitForChildrenToSync,
		recorder)

	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)

	if err := registrationController.Run(threads, stop); err != nil {
		logger.Fatalf("Error running controller: %s", err.Error())
	}
}

func startApplicationController(client kubernetes.Interface, radixClient radixclient.Interface, recorder record.EventRecorder, stop <-chan struct{}, secretProviderClient secretProviderClient.Interface, threads int) {

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, resyncPeriod)
	radixInformerFactory := informers.NewSharedInformerFactory(radixClient, resyncPeriod)

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
		func(syncedOk bool) {}, // Not interested in getting notifications of synced)
	)

	waitForChildrenToSync := true
	applicationController := application.NewController(
		client,
		radixClient,
		&handler,
		kubeInformerFactory,
		radixInformerFactory,
		waitForChildrenToSync,
		recorder)

	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)

	if err := applicationController.Run(threads, stop); err != nil {
		logger.Fatalf("Error running controller: %s", err.Error())
	}
}

func startEnvironmentController(client kubernetes.Interface, radixClient radixclient.Interface, recorder record.EventRecorder, stop <-chan struct{}, secretProviderClient secretProviderClient.Interface, threads int) {

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, resyncPeriod)
	radixInformerFactory := informers.NewSharedInformerFactory(radixClient, resyncPeriod)

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

	waitForChildrenToSync := true
	environmentController := environment.NewController(
		client,
		radixClient,
		&handler,
		kubeInformerFactory,
		radixInformerFactory,
		waitForChildrenToSync,
		recorder)

	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)

	if err := environmentController.Run(threads, stop); err != nil {
		logger.Fatalf("Error running controller: %s", err.Error())
	}
}

func startDeploymentController(client kubernetes.Interface, radixClient radixclient.Interface, prometheusOperatorClient monitoring.Interface, recorder record.EventRecorder, stop <-chan struct{}, secretProviderClient secretProviderClient.Interface, threads int) {

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, resyncPeriod)
	radixInformerFactory := informers.NewSharedInformerFactory(radixClient, resyncPeriod)

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
		deployment.WithOAuth2DefaultConfig(oauthDefaultConfig),
		deployment.WithIngressConfiguration(ingressConfiguration),
		deployment.WithOAuth2ProxyDockerImage(oauth2DockerImage),
	)

	waitForChildrenToSync := true
	deployController := deployment.NewController(
		client,
		radixClient,
		handler,
		kubeInformerFactory,
		radixInformerFactory,
		waitForChildrenToSync,
		recorder)

	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)

	if err := deployController.Run(threads, stop); err != nil {
		logger.Fatalf("Error running controller: %s", err.Error())
	}
}

func startJobController(client kubernetes.Interface, radixClient radixclient.Interface, recorder record.EventRecorder, stop <-chan struct{}, secretProviderClient secretProviderClient.Interface, threads int, config *jobUtil.Config) {
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, resyncPeriod)
	radixInformerFactory := informers.NewSharedInformerFactory(radixClient, resyncPeriod)

	kubeUtil, _ := kube.NewWithListers(
		client,
		radixClient,
		secretProviderClient,
		kubeInformerFactory,
		radixInformerFactory,
	)

	handler := job.NewHandler(client, kubeUtil, radixClient, config, func(syncedOk bool) {}) // Not interested in getting notifications of synced)

	waitForChildrenToSync := true
	jobController := job.NewController(client, radixClient, &handler, kubeInformerFactory, radixInformerFactory, waitForChildrenToSync, recorder)

	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)

	if err := jobController.Run(threads, stop); err != nil {
		logger.Fatalf("Error running controller: %s", err.Error())
	}
}

func startAlertController(client kubernetes.Interface, radixClient radixclient.Interface, prometheusOperatorClient monitoring.Interface, recorder record.EventRecorder, stop <-chan struct{}, secretProviderClient secretProviderClient.Interface, threads int) {
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, resyncPeriod)
	radixInformerFactory := informers.NewSharedInformerFactory(radixClient, resyncPeriod)

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

	waitForChildrenToSync := true
	alertController := alert.NewController(
		client,
		radixClient,
		handler,
		kubeInformerFactory,
		radixInformerFactory,
		waitForChildrenToSync,
		recorder)

	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)

	if err := alertController.Run(threads, stop); err != nil {
		logger.Fatalf("Error running controller: %s", err.Error())
	}
}

func startBatchController(client kubernetes.Interface, radixClient radixclient.Interface, recorder record.EventRecorder, stop <-chan struct{}, secretProviderClient secretProviderClient.Interface, threads int) {
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, resyncPeriod)
	radixInformerFactory := informers.NewSharedInformerFactory(radixClient, resyncPeriod)

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

	waitForChildrenToSync := true
	batchController := batch.NewController(
		client,
		radixClient,
		handler,
		kubeInformerFactory,
		radixInformerFactory,
		waitForChildrenToSync,
		recorder)

	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)

	if err := batchController.Run(threads, stop); err != nil {
		logger.Fatalf("Error running controller: %s", err.Error())
	}
}

func loadIngressConfigFromMap(kubeutil *kube.Kube) (deploymentAPI.IngressConfiguration, error) {
	config := deploymentAPI.IngressConfiguration{}
	configMap, err := kubeutil.GetConfigMap(metav1.NamespaceDefault, ingressConfigurationMap)
	if err != nil {
		return config, nil
	}

	err = yaml.Unmarshal([]byte(configMap.Data["ingressConfiguration"]), &config)
	if err != nil {
		return config, err
	}
	return config, nil
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
func Healthz(writer http.ResponseWriter, r *http.Request) {
	health := HealthStatus{
		Status: http.StatusOK,
	}

	response, err := json.Marshal(health)

	if err != nil {
		http.Error(writer, "Error while retrieving HealthStatus", http.StatusInternalServerError)
		logger.Errorf("Could not serialize HealthStatus: %v", err)
		return
	}

	fmt.Fprintf(writer, "%s", response)
}

func setLogLevel(logLevel string) {
	switch logLevel {
	case string(config.LogLevelDebug):
		log.SetLevel(log.DebugLevel)
	case string(config.LogLevelError):
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}
}
