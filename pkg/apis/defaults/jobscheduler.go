package defaults

import "fmt"

const RadixJobSchedulerPortName = "scheduler-port"
const RadixJobTimeLimitSeconds = 43200 // 12 hours

// GetJobAuxKubeDeployName Get the aux kube deployment name for a job component
func GetJobAuxKubeDeployName(jobName string) string {
	return fmt.Sprintf("%s-aux", jobName)
}
