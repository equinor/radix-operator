package defaults

const (
	// RadixPipelineActionPrepare Pipeline action to copy radixconfig.yaml to a ConfigMap, load Tekton pipelines and tasks from yaml files
	RadixPipelineActionPrepare = "prepare"
	// PipelineConfigMapContent RadixApplication content from the prepare pipeline job
	PipelineConfigMapContent = "content"
	// PipelineConfigMapBuildContext BuildContext content from the prepare pipeline job
	PipelineConfigMapBuildContext = "buildContext"
	//PipelineNameAnnotation Original pipeline name, overridden by unique generated name
	PipelineNameAnnotation = "radix.equinor.com/tekton-pipeline-name"
	//DefaultPipelineFileName Default pipeline file name. It can be overridden in the Radix config file
	DefaultPipelineFileName = "tekton/pipeline.yaml"
)
