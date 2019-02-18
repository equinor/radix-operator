package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"

	monitoring "github.com/coreos/prometheus-operator/pkg/client/monitoring"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/application"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/equinor/radix-operator/radix-operator/deployment"
	"github.com/equinor/radix-operator/radix-operator/registration"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"

	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	kubeinformers "k8s.io/client-go/informers"
)

const (
	resyncPeriod = 0
	threadiness  = 1
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

	client, radixClient, prometheusOperatorClient := utils.GetKubernetesClient()

	stop := make(chan struct{})
	defer close(stop)

	go startMetricsServer(stop)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, resyncPeriod)
	radixInformerFactory := informers.NewSharedInformerFactory(radixClient, resyncPeriod)
	eventRecorder := common.NewEventRecorder("Radix controller", client.CoreV1().Events(""))

	go startRegistrationController(client, radixClient, kubeInformerFactory, radixInformerFactory, eventRecorder, stop)
	go startApplicationController(client, radixClient, radixInformerFactory, eventRecorder, stop)
	go startDeploymentController(client, radixClient, prometheusOperatorClient, radixInformerFactory, eventRecorder, stop)

	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, syscall.SIGTERM)
	signal.Notify(sigTerm, syscall.SIGINT)
	<-sigTerm
}

func startRegistrationController(
	client kubernetes.Interface,
	radixClient radixclient.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	radixInformerFactory informers.SharedInformerFactory,
	recorder record.EventRecorder,
	stop <-chan struct{}) {

	handler := registration.NewRegistrationHandler(
		client,
		radixClient)

	registrationController := registration.NewController(
		client,
		radixClient,
		&handler,
		radixInformerFactory.Radix().V1().RadixRegistrations(),
		kubeInformerFactory.Core().V1().Namespaces(),
		recorder)

	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)

	if err := registrationController.Run(1, stop); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}

func startApplicationController(
	client kubernetes.Interface,
	radixClient radixclient.Interface,
	radixInformerFactory informers.SharedInformerFactory,
	recorder record.EventRecorder,
	stop <-chan struct{}) {

	handler := application.NewApplicationHandler(client, radixClient)
	applicationController := application.NewApplicationController(
		client,
		radixClient,
		&handler,
		radixInformerFactory.Radix().V1().RadixApplications(),
		recorder)

	radixInformerFactory.Start(stop)

	if err := applicationController.Run(1, stop); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}

func startDeploymentController(
	client kubernetes.Interface,
	radixClient radixclient.Interface,
	prometheusOperatorClient monitoring.Interface,
	radixInformerFactory informers.SharedInformerFactory,
	recorder record.EventRecorder,
	stop <-chan struct{}) {

	handler := deployment.NewDeployHandler(client, radixClient, prometheusOperatorClient)
	deployController := deployment.NewDeployController(
		client,
		radixClient,
		&handler,
		radixInformerFactory.Radix().V1().RadixDeployments(),
		recorder)

	radixInformerFactory.Start(stop)

	if err := deployController.Run(1, stop); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
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
