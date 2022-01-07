package v1

type RadixCommonEnvironmentConfig interface {
	GetEnvironment() string
	GetVariables() EnvVarsMap
	GetSecretRefs() RadixSecretRefs
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

func (config RadixJobComponentEnvironmentConfig) GetEnvironment() string {
	return config.Environment
}

func (config RadixJobComponentEnvironmentConfig) GetSecretRefs() RadixSecretRefs {
	return config.SecretRefs
}

func (config RadixJobComponentEnvironmentConfig) GetVariables() EnvVarsMap {
	return config.Variables
}
