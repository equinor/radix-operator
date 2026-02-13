package model

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Step Generic interface for any Step implementation
type Step interface {
	Init(context.Context, kubernetes.Interface, radixclient.Interface, client.Client, tektonclient.Interface, *radixv1.RadixRegistration)

	ImplementationForType() pipeline.StepType
	ErrorMsg(error) string
	SucceededMsg() string
	Run(context.Context, *PipelineInfo) error

	GetAppName() string
	GetRegistration() *radixv1.RadixRegistration
	GetKubeClient() kubernetes.Interface
	GetRadixClient() radixclient.Interface
	GetTektonClient() tektonclient.Interface
	GetDynamicClient() client.Client
}

// DefaultStepImplementation Struct to hold the data common to all step implementations
type DefaultStepImplementation struct {
	StepType       pipeline.StepType
	kubeClient     kubernetes.Interface
	radixClient    radixclient.Interface
	kubeUtil       *kube.Kube
	dynamicClient  client.Client
	tektonClient   tektonclient.Interface
	rr             *radixv1.RadixRegistration
	ErrorMessage   string
	SuccessMessage string
	Error          error
}

// Init Initialize step
func (step *DefaultStepImplementation) Init(ctx context.Context, kubeClient kubernetes.Interface, radixClient radixclient.Interface, dynamicClient client.Client, tektonClient tektonclient.Interface, rr *radixv1.RadixRegistration) {
	step.rr = rr
	step.kubeClient = kubeClient
	step.radixClient = radixClient
	step.dynamicClient = dynamicClient
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

// GetAppName The name of the Radix application
func (step *DefaultStepImplementation) GetAppID() radixv1.ULID {
	return step.rr.Spec.AppID
}

// GetRegistration The Radix registration
func (step *DefaultStepImplementation) GetRegistration() *radixv1.RadixRegistration {
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

// GetDynamicClient Get dynamic client
func (step *DefaultStepImplementation) GetDynamicClient() client.Client {
	return step.dynamicClient
}
