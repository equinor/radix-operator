package defaults

import (
	"strconv"
)

const (
	// RegistrationControllerThreadsEnvironmentVariable Number of parallel threads for registration controller
	RegistrationControllerThreadsEnvironmentVariable = "REGISTRATION_CONTROLLER_THREADS"

	// ApplicationControllerThreadsEnvironmentVariable Number of parallel threads for application controller
	ApplicationControllerThreadsEnvironmentVariable = "APPLICATION_CONTROLLER_THREADS"

	// EnvironmentControllerThreadsEnvironmentVariable Number of parallel threads for environment controller
	EnvironmentControllerThreadsEnvironmentVariable = "ENVIRONMENT_CONTROLLER_THREADS"

	// DeploymentControllerThreadsEnvironmentVariable Number of parallel threads for deployment controller
	DeploymentControllerThreadsEnvironmentVariable = "DEPLOYMENT_CONTROLLER_THREADS"

	// JobControllerThreadsEnvironmentVariable Number of parallel threads for job controller
	JobControllerThreadsEnvironmentVariable = "JOB_CONTROLLER_THREADS"

	// AlertControllerThreadsEnvironmentVariable Number of parallel threads for alert controller
	AlertControllerThreadsEnvironmentVariable = "ALERT_CONTROLLER_THREADS"

	// KubeClientRateLimitBurstEnvironmentVariable Rate limit for burst for k8s client
	KubeClientRateLimitBurstEnvironmentVariable = "KUBE_CLIENT_RATE_LIMIT_BURST"

	// KubeClientRateLimitQpsEnvironmentVariable Rate limit for queries per second for k8s client
	KubeClientRateLimitQpsEnvironmentVariable = "KUBE_CLIENT_RATE_LIMIT_QPS"
)

// GetRegistrationControllerThreads returns number of registration controller threads
func GetRegistrationControllerThreads() (int, error) {
	return GetIntEnvVar(RegistrationControllerThreadsEnvironmentVariable)
}

// GetApplicationControllerThreads returns number of application controller threads
func GetApplicationControllerThreads() (int, error) {
	return GetIntEnvVar(ApplicationControllerThreadsEnvironmentVariable)
}

// GetEnvironmentControllerThreads returns number of environment controller threads
func GetEnvironmentControllerThreads() (int, error) {
	return GetIntEnvVar(EnvironmentControllerThreadsEnvironmentVariable)
}

// GetDeploymentControllerThreads returns number of deployment controller threads
func GetDeploymentControllerThreads() (int, error) {
	return GetIntEnvVar(DeploymentControllerThreadsEnvironmentVariable)
}

// GetJobControllerThreads returns number of job controller threads
func GetJobControllerThreads() (int, error) {
	return GetIntEnvVar(JobControllerThreadsEnvironmentVariable)
}

// GetAlertControllerThreads returns number of alert controller threads
func GetAlertControllerThreads() (int, error) {
	return GetIntEnvVar(AlertControllerThreadsEnvironmentVariable)
}

// GetKubeClientRateLimitBurst returns rate limit for burst for k8s client
func GetKubeClientRateLimitBurst() (int, error) {
	return GetIntEnvVar(KubeClientRateLimitBurstEnvironmentVariable)
}

// GetKubeClientRateLimitQps returns rate limit for queries per second for k8s client
func GetKubeClientRateLimitQps() (float32, error) {
	envVarStr, err := GetEnvVar(KubeClientRateLimitQpsEnvironmentVariable)
	if err != nil {
		return 0, err
	}

	envVarFloat64, err := strconv.ParseFloat(envVarStr, 32)
	if err != nil {
		return 0, err
	}

	envVarFloat := float32(envVarFloat64)
	return envVarFloat, nil
}
