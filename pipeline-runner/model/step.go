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
	WithRadixRegistration(*v1.RadixRegistration) Step
	WithRadixApplicationConfig(*v1.RadixApplication) Step
	WithKubeClient(kubernetes.Interface) Step
	WithRadixClient(radixclient.Interface) Step
	WithKubeUtil(*kube.Kube) Step
	WithPrometheusOperatorClient(monitoring.Interface) Step

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

// WithRadixRegistration Setter for radix registration
func (step *DefaultStepImplementation) WithRadixRegistration(registration *v1.RadixRegistration) Step {
	step.Registration = registration
	return step
}

// WithRadixApplicationConfig Setter for radix config
func (step *DefaultStepImplementation) WithRadixApplicationConfig(applicationConfig *v1.RadixApplication) Step {
	step.ApplicationConfig = applicationConfig
	return step
}

// WithKubeClient Setter for kubernetes client
func (step *DefaultStepImplementation) WithKubeClient(kubeclient kubernetes.Interface) Step {
	step.Kubeclient = kubeclient
	return step
}

// WithRadixClient Setter for radix client
func (step *DefaultStepImplementation) WithRadixClient(radixclient radixclient.Interface) Step {
	step.Radixclient = radixclient
	return step
}

// WithKubeUtil Setter for kubernetes utility
func (step *DefaultStepImplementation) WithKubeUtil(kubeutil *kube.Kube) Step {
	step.Kubeutil = kubeutil
	return step
}

// WithPrometheusOperatorClient Setter for prom client
func (step *DefaultStepImplementation) WithPrometheusOperatorClient(prometheusOperatorClient monitoring.Interface) Step {
	step.PrometheusOperatorClient = prometheusOperatorClient
	return step
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
