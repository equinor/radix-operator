package model

import (
	"github.com/coreos/prometheus-operator/pkg/client/monitoring"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

// Step Generic interface for any Step implementation
type Step interface {
	Init(kubernetes.Interface, radixclient.Interface, *kube.Kube, monitoring.Interface)

	ImplementationForType() pipeline.StepType
	ErrorMsg(*PipelineInfo, error) string
	SucceededMsg(*PipelineInfo) string
	Run(*PipelineInfo) error
}

// DefaultStepImplementation Struct to hold the data common to all step implementations
type DefaultStepImplementation struct {
	StepType                 pipeline.StepType
	Kubeclient               kubernetes.Interface
	Radixclient              radixclient.Interface
	Kubeutil                 *kube.Kube
	PrometheusOperatorClient monitoring.Interface

	ErrorMessage   string
	SuccessMessage string
	Error          error
}

// Init Initialize step
func (step *DefaultStepImplementation) Init(
	kubeclient kubernetes.Interface, radixclient radixclient.Interface, kubeutil *kube.Kube, prometheusOperatorClient monitoring.Interface) {
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
func (step *DefaultStepImplementation) ErrorMsg(pipelineInfo *PipelineInfo, err error) string {
	return step.ErrorMessage
}

// SucceededMsg Default implementation
func (step *DefaultStepImplementation) SucceededMsg(pipelineInfo *PipelineInfo) string {
	return step.SuccessMessage
}

// Run Default implementation
func (step *DefaultStepImplementation) Run(pipelineInfo *PipelineInfo) error {
	return step.Error
}
