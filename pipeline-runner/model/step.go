package model

import (
	"context"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"k8s.io/client-go/kubernetes"
)

// Step Generic interface for any Step implementation
type Step interface {
	Init(context.Context, kubernetes.Interface, radixclient.Interface, *kube.Kube, monitoring.Interface, tektonclient.Interface, *v1.RadixRegistration)

	ImplementationForType() pipeline.StepType
	ErrorMsg(error) string
	SucceededMsg() string
	Run(context.Context, *PipelineInfo) error

	GetAppName() string
	GetRegistration() *v1.RadixRegistration
	GetKubeClient() kubernetes.Interface
	GetRadixClient() radixclient.Interface
	GetKubeUtil() *kube.Kube
	GetPrometheusOperatorClient() monitoring.Interface
}

// DefaultStepImplementation Struct to hold the data common to all step implementations
type DefaultStepImplementation struct {
	StepType                 pipeline.StepType
	kubeClient               kubernetes.Interface
	radixClient              radixclient.Interface
	kubeUtil                 *kube.Kube
	prometheusOperatorClient monitoring.Interface
	tektonClient             tektonclient.Interface
	rr                       *v1.RadixRegistration
	ErrorMessage             string
	SuccessMessage           string
	Error                    error
}

// Init Initialize step
func (step *DefaultStepImplementation) Init(ctx context.Context, kubeClient kubernetes.Interface, radixClient radixclient.Interface, kubeUtil *kube.Kube, prometheusOperatorClient monitoring.Interface, tektonClient tektonclient.Interface, rr *v1.RadixRegistration) {
	step.rr = rr
	step.kubeClient = kubeClient
	step.radixClient = radixClient
	step.kubeUtil = kubeUtil
	step.prometheusOperatorClient = prometheusOperatorClient
	step.tektonClient = tektonClient
}

// ImplementationForType The type of the step
func (step *DefaultStepImplementation) ImplementationForType() pipeline.StepType {
	return step.StepType
}

// ErrorMsg Error message
func (step *DefaultStepImplementation) ErrorMsg(err error) string {
	return step.ErrorMessage
}

// SucceededMsg Success message
func (step *DefaultStepImplementation) SucceededMsg() string {
	return step.SuccessMessage
}

// Run the step
func (step *DefaultStepImplementation) Run(_ context.Context, pipelineInfo *PipelineInfo) error {
	return step.Error
}

// GetAppName The name of the Radix application
func (step *DefaultStepImplementation) GetAppName() string {
	return step.rr.Name
}

// GetRegistration The Radix registration
func (step *DefaultStepImplementation) GetRegistration() *v1.RadixRegistration {
	return step.rr
}

// GetKubeClient Gets Kubernetes client
func (step *DefaultStepImplementation) GetKubeClient() kubernetes.Interface {
	return step.kubeClient
}

// GetRadixClient Gets Radix client
func (step *DefaultStepImplementation) GetRadixClient() radixclient.Interface {
	return step.radixClient
}

// GetTektonClient Gets Tekton client
func (step *DefaultStepImplementation) GetTektonClient() tektonclient.Interface {
	return step.tektonClient
}

// GetKubeUtil Gets Kubernetes utils
func (step *DefaultStepImplementation) GetKubeUtil() *kube.Kube {
	return step.kubeUtil
}

// GetPrometheusOperatorClient Get Prometheus client
func (step *DefaultStepImplementation) GetPrometheusOperatorClient() monitoring.Interface {
	return step.prometheusOperatorClient
}
