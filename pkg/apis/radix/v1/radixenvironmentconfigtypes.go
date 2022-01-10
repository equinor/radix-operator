package v1

type RadixCommonEnvironmentConfig interface {
	GetEnvironment() string
	GetVariables() EnvVarsMap
	GetSecretRefs() RadixSecretRefs
	GetResources() ResourceRequirements
	GetNode() RadixNode
	GetImageTagName() string
}

func (config RadixEnvironmentConfig) GetEnvironment() string {
	return config.Environment
}

func (config RadixEnvironmentConfig) GetSecretRefs() RadixSecretRefs {
	return config.SecretRefs
}

func (config RadixEnvironmentConfig) GetVariables() EnvVarsMap {
	return config.Variables
}

func (config RadixEnvironmentConfig) GetResources() ResourceRequirements {
	return config.Resources
}

func (config RadixEnvironmentConfig) GetNode() RadixNode {
	return config.Node
}

func (config RadixEnvironmentConfig) GetImageTagName() string {
	return config.ImageTagName
}

func (config RadixJobComponentEnvironmentConfig) GetEnvironment() string {
	return config.Environment
}

func (config RadixJobComponentEnvironmentConfig) GetSecretRefs() RadixSecretRefs {
	return config.SecretRefs
}

func (config RadixJobComponentEnvironmentConfig) GetVariables() EnvVarsMap {
	return config.Variables
}

func (config RadixJobComponentEnvironmentConfig) GetResources() ResourceRequirements {
	return config.Resources
}

func (config RadixJobComponentEnvironmentConfig) GetNode() RadixNode {
	return config.Node
}

func (config RadixJobComponentEnvironmentConfig) GetImageTagName() string {
	return config.ImageTagName
}
