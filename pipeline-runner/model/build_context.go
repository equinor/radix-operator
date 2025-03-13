package model

// EnvironmentToBuild An application environment to be built
type EnvironmentToBuild struct {
	// Environment name
	Environment string
	// Components Names of components, which need to be built
	Components []string
}

// EnvironmentSubPipelineToRun An application environment sub-pipeline to be run
type EnvironmentSubPipelineToRun struct {
	// Environment name
	Environment string
	// PipelineFile Name of a sub-pipeline file, which need to be run
	PipelineFile string
}

// BuildContext Provide optional build instruction from the prepare job to the pipeline job
type BuildContext struct {
	// EnvironmentsToBuild List of environments with component names, which need to be built
	EnvironmentsToBuild []EnvironmentToBuild
	// ChangedRadixConfig Radix Config file was changed
	ChangedRadixConfig bool
	// EnvironmentSubPipelinesToRun Sub-pipeline pipeline file named, if they are configured to be run
	EnvironmentSubPipelinesToRun []EnvironmentSubPipelineToRun
}
