package defaults

const (
	// RadixPipelineActionPrepare Pipeline action to copy radixconfig.yaml to a ConfigMap, load Tekton pipelines and tasks from yaml files
	RadixPipelineActionPrepare = "prepare"
	// RadixPipelineActionRun Pipeline action to run Tekton PipelineRun for pipelines and tasks, prepared during the RadixPipelineActionPrepare action
	RadixPipelineActionRun = "run"
	// PipelineConfigMapContent RadixApplication content from the prepare pipeline job
	PipelineConfigMapContent = "content"
	// PipelineConfigMapBuildContext PrepareBuildContext content from the prepare pipeline job
	PipelineConfigMapBuildContext = "buildContext"
)
