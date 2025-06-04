package model

// EnvironmentToBuild An application environment to be built
type EnvironmentToBuild struct {
	// Environment name
	Environment string
	// Components Names of components, which need to be built
	Components []string
}

// BuildContext Provide optional build instruction from the prepare job to the pipeline job
type BuildContext struct {
	// EnvironmentsToBuild List of environments with component names, which need to be built
	EnvironmentsToBuild []EnvironmentToBuild
}
