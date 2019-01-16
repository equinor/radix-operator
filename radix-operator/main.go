package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	log "github.com/Sirupsen/logrus"

	monitoring "github.com/coreos/prometheus-operator/pkg/client/monitoring"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/client/monitoring/v1"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/deployment"
	"github.com/equinor/radix-operator/radix-operator/registration"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var logger *log.Entry

var (
	operatorDate     string
	operatorCommitid string
	operatorBranch   string
)

func init() {
	if operatorCommitid == "" {
		operatorCommitid = "no commitid"
	}

	if operatorBranch == "" {
		operatorBranch = "no branch"
	}

	if operatorDate == "" {
		operatorDate = "(Mon YYYY)"
	}
}

func main() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": "main"})

	logger.Infof("Starting Radix Operator from commit %s on branch %s built %s", operatorCommitid, operatorBranch, operatorDate)

	client, radixClient, prometheusOperatorClient := getKubernetesClient()

	stop := make(chan struct{})
	defer close(stop)

	go startMetricsServer(stop)

	startRegistrationController(client, radixClient, stop)
	startDeploymentController(client, radixClient, prometheusOperatorClient, stop)

	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, syscall.SIGTERM)
	signal.Notify(sigTerm, syscall.SIGINT)
	<-sigTerm
}

func startRegistrationController(client kubernetes.Interface, radixClient radixclient.Interface, stop <-chan struct{}) {
	handler := registration.NewRegistrationHandler(client)
	registrationController := registration.NewController(client, radixClient, &handler)

	go registrationController.Run(stop)
}

func startDeploymentController(client kubernetes.Interface, radixClient radixclient.Interface, prometheusOperatorClient monitoring.Interface, stop <-chan struct{}) {
	deployHandler := deployment.NewDeployHandler(client, radixClient, prometheusOperatorClient)

	deployController := deployment.NewDeployController(client, radixClient, &deployHandler)
	go deployController.Run(stop)
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
	if err := srv.Shutdown(nil); err != nil {
		panic(err)
	}
}

type HealthStatus struct {
	Status int
}

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

func getKubernetesClient() (kubernetes.Interface, radixclient.Interface, monitoring.Interface) {
	kubeConfigPath := os.Getenv("HOME") + "/.kube/config"
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			logger.Fatalf("getClusterConfig InClusterConfig: %v", err)
		}
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatalf("getClusterConfig k8s client: %v", err)
	}

	radixClient, err := radixclient.NewForConfig(config)
	if err != nil {
		logger.Fatalf("getClusterConfig radix client: %v", err)
	}

	prometheusOperatorClient, err := monitoring.NewForConfig(&monitoringv1.DefaultCrdKinds, "monitoring.coreos.com", config)
	if err != nil {
		logger.Fatalf("getClusterConfig prometheus-operator client: %v", err)
	}

	logger.Printf("Successfully constructed k8s client to API server %v", config.Host)
	return client, radixClient, prometheusOperatorClient
}
