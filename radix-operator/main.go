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

	applicationAPI "github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	deploymentAPI "github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/metrics"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/equinor/radix-operator/radix-operator/alert"
	"github.com/equinor/radix-operator/radix-operator/application"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/equinor/radix-operator/radix-operator/deployment"
	"github.com/equinor/radix-operator/radix-operator/environment"
	"github.com/equinor/radix-operator/radix-operator/job"
	"github.com/equinor/radix-operator/radix-operator/registration"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/record"
	secretProviderClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
)

const (
	resyncPeriod                  = 0
	ingressConfigurationMap       = "radix-operator-ingress-configmap"
	alertControllerThreads        = 10
	applicationControllerThreads  = 10
	deploymentControllerThreads   = 50
	environmentControllerThreads  = 10
	jobControllerThreads          = 50
	registrationControllerThreads = 10
)

var logger *log.Entry

func main() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": "main"})
	switch os.Getenv(defaults.LogLevel) {
	case "DEBUG":
		logger.Logger.SetLevel(log.DebugLevel)
	case "ERROR":
		logger.Logger.SetLevel(log.ErrorLevel)
	default:
		logger.Logger.SetLevel(log.InfoLevel)
	}
	client, radixClient, prometheusOperatorClient, secretProviderClient := utils.GetKubernetesClient()

	activeclusternameEnvVar := os.Getenv(defaults.ActiveClusternameEnvironmentVariable)
	logger.Printf("Active cluster name: %v", activeclusternameEnvVar)

	stop := make(chan struct{})
	defer close(stop)

	go startMetricsServer(stop)

	eventRecorder := common.NewEventRecorder("Radix controller", client.CoreV1().Events(""), logger)

	go startRegistrationController(client, radixClient, eventRecorder, stop, secretProviderClient)
	go startApplicationController(client, radixClient, eventRecorder, stop, secretProviderClient)
	go startEnvironmentController(client, radixClient, eventRecorder, stop, secretProviderClient)
	go startDeploymentController(client, radixClient, prometheusOperatorClient, eventRecorder, stop, secretProviderClient)
	go startJobController(client, radixClient, eventRecorder, stop, secretProviderClient)
	go startAlertController(client, radixClient, prometheusOperatorClient, eventRecorder, stop, secretProviderClient)

	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, syscall.SIGTERM)
	signal.Notify(sigTerm, syscall.SIGINT)
	<-sigTerm
}

func startRegistrationController(client kubernetes.Interface, radixClient radixclient.Interface, recorder record.EventRecorder, stop <-chan struct{}, secretProviderClient secretProviderClient.Interface) {

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

	if err := registrationController.Run(registrationControllerThreads, stop); err != nil {
		logger.Fatalf("Error running controller: %s", err.Error())
	}
}

func startApplicationController(client kubernetes.Interface, radixClient radixclient.Interface, recorder record.EventRecorder, stop <-chan struct{}, secretProviderClient secretProviderClient.Interface) {

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

	if err := applicationController.Run(applicationControllerThreads, stop); err != nil {
		logger.Fatalf("Error running controller: %s", err.Error())
	}
}

func startEnvironmentController(client kubernetes.Interface, radixClient radixclient.Interface, recorder record.EventRecorder, stop <-chan struct{}, secretProviderClient secretProviderClient.Interface) {

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

	if err := environmentController.Run(environmentControllerThreads, stop); err != nil {
		logger.Fatalf("Error running controller: %s", err.Error())
	}
}

func startDeploymentController(client kubernetes.Interface, radixClient radixclient.Interface, prometheusOperatorClient monitoring.Interface, recorder record.EventRecorder, stop <-chan struct{}, secretProviderClient secretProviderClient.Interface) {

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
		deployment.WithForceRunAsNonRootFromEnvVar(defaults.RadixDeploymentForceNonRootContainers),
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

	if err := deployController.Run(deploymentControllerThreads, stop); err != nil {
		logger.Fatalf("Error running controller: %s", err.Error())
	}
}

func startJobController(client kubernetes.Interface, radixClient radixclient.Interface, recorder record.EventRecorder, stop <-chan struct{}, secretProviderClient secretProviderClient.Interface) {

	syncJobStatusMetrics(radixClient)
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, resyncPeriod)
	radixInformerFactory := informers.NewSharedInformerFactory(radixClient, resyncPeriod)

	kubeUtil, _ := kube.NewWithListers(
		client,
		radixClient,
		secretProviderClient,
		kubeInformerFactory,
		radixInformerFactory,
	)

	handler := job.NewHandler(client,
		kubeUtil,
		radixClient,
		func(syncedOk bool) {}) // Not interested in getting notifications of synced)

	waitForChildrenToSync := true
	jobController := job.NewController(
		client,
		radixClient,
		&handler,
		kubeInformerFactory,
		radixInformerFactory,
		waitForChildrenToSync,
		recorder)

	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)

	if err := jobController.Run(jobControllerThreads, stop); err != nil {
		logger.Fatalf("Error running controller: %s", err.Error())
	}
}

func startAlertController(client kubernetes.Interface, radixClient radixclient.Interface, prometheusOperatorClient monitoring.Interface, recorder record.EventRecorder, stop <-chan struct{}, secretProviderClient secretProviderClient.Interface) {

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

	if err := alertController.Run(alertControllerThreads, stop); err != nil {
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

func syncJobStatusMetrics(radixClient radixclient.Interface) error {
	logger.Info("Restore vulnerability scan metrics from build job.")
	radixJobs, err := radixClient.RadixV1().RadixJobs("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	logger.Debugf("Found %d build Radix jobs.", len(radixJobs.Items))
	radixJobMap := make(map[string]v1.RadixJob)
	for _, radixJob := range radixJobs.Items {
		if radixJob.Status.Condition != v1.JobSucceeded {
			continue
		}
		for _, env := range radixJob.Status.TargetEnvs {
			appEnvKey := fmt.Sprintf("%s#%s", radixJob.Namespace, env)
			cachedRadixJob, found := radixJobMap[appEnvKey]
			if !found || cachedRadixJob.Status.Ended.Before(radixJob.Status.Ended) {
				radixJobMap[appEnvKey] = radixJob
			}
		}
	}
	for _, radixJob := range radixJobMap {
		metrics.RadixJobVulnerabilityScan(&radixJob)
	}
	logger.Debugf("Completed restoring vulnerability scan metrics from build job for %d Radix jobs.", len(radixJobMap))
	return nil
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
