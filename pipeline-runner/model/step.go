package model

import (
	"github.com/coreos/prometheus-operator/pkg/client/monitoring"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

// Step Generic interface for any Step implementation
type Step interface {
	Init(kubernetes.Interface, radixclient.Interface, *kube.Kube, monitoring.Interface, *v1.RadixRegistration, *v1.RadixApplication)

	ImplementationForType() pipeline.StepType
	ErrorMsg(error) string
	SucceededMsg() string
	Run(*PipelineInfo) error

	GetAppName() string
	GetRegistration() *v1.RadixRegistration
	GetApplicationConfig() *v1.RadixApplication
	GetKubeclient() kubernetes.Interface
	GetRadixclient() radixclient.Interface
	GetKubeutil() *kube.Kube
	GetPrometheusOperatorClient() monitoring.Interface
}

// DefaultStepImplementation Struct to hold the data common to all step implementations
type DefaultStepImplementation struct {
	StepType                 pipeline.StepType
	kubeclient               kubernetes.Interface
	radixclient              radixclient.Interface
	kubeutil                 *kube.Kube
	prometheusOperatorClient monitoring.Interface
	rr                       *v1.RadixRegistration
	ra                       *v1.RadixApplication
	ErrorMessage             string
	SuccessMessage           string
	Error                    error
}

// Init Initialize step
func (step *DefaultStepImplementation) Init(
	kubeclient kubernetes.Interface, radixclient radixclient.Interface, kubeutil *kube.Kube, prometheusOperatorClient monitoring.Interface,
	rr *v1.RadixRegistration, ra *v1.RadixApplication) {
	step.rr = rr
	step.ra = ra
	step.kubeclient = kubeclient
	step.radixclient = radixclient
	step.kubeutil = kubeutil
	step.prometheusOperatorClient = prometheusOperatorClient
}

// ImplementationForType Default implementation
func (step *DefaultStepImplementation) ImplementationForType() pipeline.StepType {
	return step.StepType
}

// ErrorMsg Default implementation
func (step *DefaultStepImplementation) ErrorMsg(err error) string {
	return step.ErrorMessage
}

// SucceededMsg Default implementation
func (step *DefaultStepImplementation) SucceededMsg() string {
	return step.SuccessMessage
}

// Run Default implementation
func (step *DefaultStepImplementation) Run(pipelineInfo *PipelineInfo) error {
	return step.Error
}

// GetAppName Default implementation
func (step *DefaultStepImplementation) GetAppName() string {
	return step.rr.Name
}

// GetRegistration Default implementation
func (step *DefaultStepImplementation) GetRegistration() *v1.RadixRegistration {
	return step.rr
}

// GetApplicationConfig Default implementation
func (step *DefaultStepImplementation) GetApplicationConfig() *v1.RadixApplication {
	return step.ra
}

// GetKubeclient Default implementation
func (step *DefaultStepImplementation) GetKubeclient() kubernetes.Interface {
	return step.kubeclient
}

// GetRadixclient Default implementation
func (step *DefaultStepImplementation) GetRadixclient() radixclient.Interface {
	return step.radixclient
}

// GetKubeutil Default implementation
func (step *DefaultStepImplementation) GetKubeutil() *kube.Kube {
	return step.kubeutil
}

// GetPrometheusOperatorClient Default implementation
func (step *DefaultStepImplementation) GetPrometheusOperatorClient() monitoring.Interface {
	return step.prometheusOperatorClient
}
