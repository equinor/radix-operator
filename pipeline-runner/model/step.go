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
	Init(*v1.RadixRegistration, *v1.RadixApplication, kubernetes.Interface, radixclient.Interface, *kube.Kube, monitoring.Interface)

	ImplementationForType() pipeline.StepType
	ErrorMsg(err error) string
	SucceededMsg() string
	Run(pipelineInfo PipelineInfo) error
}

// DefaultStepImplementation Struct to hold the data common to all step implementations
type DefaultStepImplementation struct {
	StepType                 pipeline.StepType
	Registration             *v1.RadixRegistration
	ApplicationConfig        *v1.RadixApplication
	Kubeclient               kubernetes.Interface
	Radixclient              radixclient.Interface
	Kubeutil                 *kube.Kube
	PrometheusOperatorClient monitoring.Interface

	ErrorMessage   string
	SuccessMessage string
	Error          error
}

// Init Initialize step
func (step *DefaultStepImplementation) Init(registration *v1.RadixRegistration, applicationConfig *v1.RadixApplication,
	kubeclient kubernetes.Interface, radixclient radixclient.Interface, kubeutil *kube.Kube, prometheusOperatorClient monitoring.Interface) {
	step.Registration = registration
	step.ApplicationConfig = applicationConfig
	step.Kubeclient = kubeclient
	step.Radixclient = radixclient
	step.Kubeutil = kubeutil
	step.PrometheusOperatorClient = prometheusOperatorClient
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
func (step *DefaultStepImplementation) Run(pipelineInfo PipelineInfo) error {
	return step.Error
}
