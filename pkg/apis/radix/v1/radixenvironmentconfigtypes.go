package v1

type RadixCommonEnvironmentConfig interface {
	GetEnvironment() string
	GetSecretRefs() RadixSecretRefs
}

func (config RadixEnvironmentConfig) GetEnvironment() string {
	return config.Environment
}

func (config RadixEnvironmentConfig) GetSecretRefs() RadixSecretRefs {
	return config.SecretRefs
}

func (config RadixJobComponentEnvironmentConfig) GetEnvironment() string {
	return config.Environment
}

func (config RadixJobComponentEnvironmentConfig) GetSecretRefs() RadixSecretRefs {
	return config.SecretRefs
}
