package defaults

const (
	// RadixPipelineJobPipelineContainerName The container name of the Radix pipeline orchestration job
	RadixPipelineJobPipelineContainerName = "radix-pipeline"

	// DefaultRadixConfigFileName Default name for the radix configuration file
	DefaultRadixConfigFileName = "radixconfig.yaml"

	//PipelineNameAnnotation Original pipeline name, overridden by unique generated name
	PipelineNameAnnotation = "radix.equinor.com/tekton-pipeline-name"

	//PipelineTaskNameAnnotation Original pipeline task name, overridden by unique generated name
	PipelineTaskNameAnnotation = "radix.equinor.com/tekton-pipeline-task-name"
)
