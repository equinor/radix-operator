package job

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	pipelineJob "github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	conditionUtils "github.com/equinor/radix-operator/pkg/apis/utils/conditions"
	numberUtils "github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	workerImage                = "radix-pipeline"
	PRIVILEGED_CONTAINER       = false
	ALLOW_PRIVILEGE_ESCALATION = false
	RUN_AS_NON_ROOT            = true
	RUN_AS_USER                = 1000
	RUN_AS_GROUP               = 1000
	FS_GROUP                   = 1000
)

func (job *Job) createJob() error {
	namespace := job.radixJob.Namespace
	name := job.radixJob.Name

	ownerReference := GetOwnerReference(job.radixJob)
	jobConfig, err := job.getJobConfig(name)
	if err != nil {
		return err
	}

	jobConfig.OwnerReferences = ownerReference
	_, err = job.kubeclient.BatchV1().Jobs(namespace).Create(context.TODO(), jobConfig, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (job *Job) getJobConfig(name string) (*batchv1.Job, error) {
	imageTag := fmt.Sprintf("%s/%s:%s", job.radixJob.Spec.DockerRegistry, workerImage, job.radixJob.Spec.PipelineImage)

	log.Infof("Using image: %s", imageTag)

	backOffLimit := int32(0)

	appName := job.radixJob.Spec.AppName
	jobName := job.radixJob.Name

	pipeline, err := pipelineJob.GetPipelineFromName(string(job.radixJob.Spec.PipeLineType))
	if err != nil {
		return nil, err
	}

	jobCfg := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   jobName,
			Labels: getPipelineJobLabels(appName, jobName, job.radixJob.Spec, pipeline),
			Annotations: map[string]string{
				kube.RadixBranchAnnotation: job.radixJob.Spec.Build.Branch,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backOffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: conditionUtils.BoolPtr(RUN_AS_NON_ROOT),
						FSGroup:      numberUtils.Int64Ptr(FS_GROUP),
					},
					ServiceAccountName: defaults.PipelineRoleName,
					Containers: []corev1.Container{
						{
							Name:            workerImage,
							Image:           imageTag,
							ImagePullPolicy: corev1.PullAlways,
							Args:            job.getPipelineJobArguments(appName, jobName, job.radixJob.Spec, pipeline),
							SecurityContext: &corev1.SecurityContext{
								Privileged:               conditionUtils.BoolPtr(PRIVILEGED_CONTAINER),
								AllowPrivilegeEscalation: conditionUtils.BoolPtr(ALLOW_PRIVILEGE_ESCALATION),
								RunAsUser:                numberUtils.Int64Ptr(RUN_AS_USER),
								RunAsGroup:               numberUtils.Int64Ptr(RUN_AS_GROUP),
							},
						},
					},
					RestartPolicy: "Never",
				},
			},
		},
	}

	return &jobCfg, nil
}

func getUniqueJobName(image string) (string, string) {
	var jobName []string
	randomStr := strings.ToLower(utils.RandString(5))
	jobName = append(jobName, image)
	jobName = append(jobName, "-")
	jobName = append(jobName, getCurrentTimestamp())
	jobName = append(jobName, "-")
	jobName = append(jobName, randomStr)
	return strings.Join(jobName, ""), randomStr
}

func getCurrentTimestamp() string {
	t := time.Now()
	return t.Format("20060102150405") // YYYYMMDDHHMISS in Go
}

func (job *Job) getPipelineJobArguments(appName, jobName string, jobSpec v1.RadixJobSpec, pipeline *pipelineJob.Definition) []string {
	clusterType := os.Getenv(defaults.OperatorClusterTypeEnvironmentVariable)

	// Operator will never have an issue with getting clustername
	clusterName, _ := kube.GetClusterName(job.kubeutil)

	// Base arguments for all types of pipeline
	args := []string{
		fmt.Sprintf("%s=%s", defaults.RadixAppEnvironmentVariable, appName),
		fmt.Sprintf("JOB_NAME=%s", jobName),
		fmt.Sprintf("PIPELINE_TYPE=%s", pipeline.Type),

		// Pass config-to-map, builder and scanner images
		fmt.Sprintf("%s=%s", defaults.RadixConfigToMapEnvironmentVariable, os.Getenv(defaults.RadixConfigToMapEnvironmentVariable)),
		fmt.Sprintf("%s=%s", defaults.RadixImageBuilderEnvironmentVariable, os.Getenv(defaults.RadixImageBuilderEnvironmentVariable)),
		fmt.Sprintf("%s=%s", defaults.RadixImageScannerEnvironmentVariable, os.Getenv(defaults.RadixImageScannerEnvironmentVariable)),

		// Used for tagging source of image
		fmt.Sprintf("%s=%s", defaults.RadixClusterTypeEnvironmentVariable, clusterType),
		fmt.Sprintf("%s=%s", defaults.ClusternameEnvironmentVariable, clusterName),
	}

	switch pipeline.Type {
	case v1.BuildDeploy, v1.Build:
		args = append(args, fmt.Sprintf("IMAGE_TAG=%s", jobSpec.Build.ImageTag))
		args = append(args, fmt.Sprintf("BRANCH=%s", jobSpec.Build.Branch))
		args = append(args, fmt.Sprintf("COMMIT_ID=%s", jobSpec.Build.CommitID))
		args = append(args, fmt.Sprintf("PUSH_IMAGE=%s", getPushImageTag(jobSpec.Build.PushImage)))
		args = append(args, fmt.Sprintf("RADIX_FILE_NAME=%s", "/workspace/radixconfig.yaml"))
	case v1.Promote:
		args = append(args, fmt.Sprintf("DEPLOYMENT_NAME=%s", jobSpec.Promote.DeploymentName))
		args = append(args, fmt.Sprintf("FROM_ENVIRONMENT=%s", jobSpec.Promote.FromEnvironment))
		args = append(args, fmt.Sprintf("TO_ENVIRONMENT=%s", jobSpec.Promote.ToEnvironment))
	case v1.Deploy:
		args = append(args, fmt.Sprintf("TO_ENVIRONMENT=%s", jobSpec.Deploy.ToEnvironment))
		args = append(args, fmt.Sprintf("RADIX_FILE_NAME=%s", "/workspace/radixconfig.yaml"))
	}

	return args
}

func getPipelineJobLabels(appName, jobName string, jobSpec v1.RadixJobSpec, pipeline *pipelineJob.Definition) map[string]string {
	// Base labels for all types of pipeline
	labels := map[string]string{
		kube.RadixJobNameLabel: jobName,
		kube.RadixJobTypeLabel: kube.RadixJobTypeJob,
		"radix-pipeline":       string(pipeline.Type),
		"radix-app-name":       appName, // For backwards compatibility. Remove when cluster is migrated
		kube.RadixAppLabel:     appName,
	}

	switch pipeline.Type {
	case v1.BuildDeploy, v1.Build:
		labels[kube.RadixImageTagLabel] = jobSpec.Build.ImageTag
		labels[kube.RadixCommitLabel] = jobSpec.Build.CommitID
	}

	return labels
}

func getPushImageTag(pushImage bool) string {
	if pushImage {
		return "1"
	}

	return "0"
}

func getJobConditionFromJobStatus(jobStatus batchv1.JobStatus) v1.RadixJobCondition {
	var status v1.RadixJobCondition

	if jobStatus.Failed > 0 {
		status = v1.JobFailed

	} else if jobStatus.Active > 0 {
		status = v1.JobRunning

	} else if jobStatus.Succeeded > 0 {
		status = v1.JobSucceeded

	}

	return status
}
