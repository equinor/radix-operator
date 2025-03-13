package runpipeline

import (
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/wait"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

// Context of the pipeline
type Context interface {
	// RunPipelinesJob un the job, which creates Tekton PipelineRun-s
	RunPipelinesJob() error
	// GetPipelineInfo Get pipeline info
	GetPipelineInfo() *model.PipelineInfo
	// GetHash Hash, common for all pipeline Kubernetes object names
	GetHash() string
	// GetKubeClient Kubernetes client
	GetKubeClient() kubernetes.Interface
	// GetTektonClient Tekton client
	GetTektonClient() tektonclient.Interface
	// GetRadixApplication Gets the RadixApplication, loaded from the config-map
	GetRadixApplication() *radixv1.RadixApplication
	// GetPipelineRunsWaiter Returns a waiter that returns when all pipelineruns have completed
	GetPipelineRunsWaiter() wait.PipelineRunsCompletionWaiter
	// GetEnvVars Gets build env vars
	GetEnvVars(envName string) radixv1.EnvVarsMap
	// SetPipelineTargetEnvironments Set target environments for the pipeline job
	SetPipelineTargetEnvironments(environments []string)
}
