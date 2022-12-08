package job

import (
	"context"
	"fmt"
	"os"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	pipelineJob "github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	conditionUtils "github.com/equinor/radix-operator/pkg/apis/utils/conditions"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	numberUtils "github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

const (
	workerImage                = "radix-pipeline"
	PRIVILEGED_CONTAINER       = false
	ALLOW_PRIVILEGE_ESCALATION = false
	RUN_AS_NON_ROOT            = true
	RUN_AS_USER                = 1000
	RUN_AS_GROUP               = 1000
	FS_GROUP                   = 1000
	// ResultContent of the pipeline job, passed via ConfigMap as v1.RadixJobResult structure
	ResultContent = "ResultContent"
)

func (job *Job) createPipelineJob() error {
	namespace := job.radixJob.Namespace

	ownerReference := GetOwnerReference(job.radixJob)
	jobConfig, err := job.getPipelineJobConfig()
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

func (job *Job) getPipelineJobConfig() (*batchv1.Job, error) {
	imageTag := fmt.Sprintf("%s/%s:%s", job.radixJob.Spec.DockerRegistry, workerImage, job.radixJob.Spec.PipelineImage)
	log.Infof("Using image: %s", imageTag)

	backOffLimit := int32(0)
	appName := job.radixJob.Spec.AppName
	jobName := job.radixJob.Name

	pipeline, err := pipelineJob.GetPipelineFromName(string(job.radixJob.Spec.PipeLineType))
	if err != nil {
		return nil, err
	}

	containerArguments, err := job.getPipelineJobArguments(appName, jobName, job.radixJob.Spec, pipeline)
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
					ServiceAccountName: defaults.PipelineServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:            defaults.RadixPipelineJobPipelineContainerName,
							Image:           imageTag,
							ImagePullPolicy: corev1.PullAlways,
							Args:            containerArguments,
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

func (job *Job) getPipelineJobArguments(appName, jobName string, jobSpec v1.RadixJobSpec, pipeline *pipelineJob.Definition) ([]string, error) {
	clusterType := os.Getenv(defaults.OperatorClusterTypeEnvironmentVariable)
	radixZone := os.Getenv(defaults.RadixZoneEnvironmentVariable)
	useImageBuilderCache := os.Getenv(defaults.RadixUseCacheEnvironmentVariable)

	clusterName, err := job.kubeutil.GetClusterName()
	if err != nil {
		return nil, err
	}
	containerRegistry, err := job.kubeutil.GetContainerRegistry()
	if err != nil {
		return nil, err
	}
	subscriptionId, err := job.kubeutil.GetSubscriptionId()
	if err != nil {
		return nil, err
	}

	// Base arguments for all types of pipeline
	args := []string{
		fmt.Sprintf("%s=%s", defaults.RadixAppEnvironmentVariable, appName),
		fmt.Sprintf("%s=%s", defaults.RadixPipelineJobEnvironmentVariable, jobName),
		fmt.Sprintf("%s=%s", defaults.RadixPipelineTypeEnvironmentVariable, pipeline.Type),

		// Pass tekton and builder images
		fmt.Sprintf("%s=%s", defaults.RadixTektonPipelineImageEnvironmentVariable, os.Getenv(defaults.RadixTektonPipelineImageEnvironmentVariable)),
		fmt.Sprintf("%s=%s", defaults.RadixImageBuilderEnvironmentVariable, os.Getenv(defaults.RadixImageBuilderEnvironmentVariable)),

		// Used for tagging source of image
		fmt.Sprintf("%s=%s", defaults.RadixClusterTypeEnvironmentVariable, clusterType),
		fmt.Sprintf("%s=%s", defaults.RadixZoneEnvironmentVariable, radixZone),
		fmt.Sprintf("%s=%s", defaults.ClusternameEnvironmentVariable, clusterName),
		fmt.Sprintf("%s=%s", defaults.ContainerRegistryEnvironmentVariable, containerRegistry),
		fmt.Sprintf("%s=%s", defaults.AzureSubscriptionIdEnvironmentVariable, subscriptionId),
	}

	radixConfigFullName := jobSpec.RadixConfigFullName
	if len(radixConfigFullName) == 0 {
		radixConfigFullName = fmt.Sprintf("%s/%s", git.Workspace, defaults.DefaultRadixConfigFileName)
	}
	switch pipeline.Type {
	case v1.BuildDeploy, v1.Build:
		args = append(args, fmt.Sprintf("%s=%s", defaults.RadixImageTagEnvironmentVariable, jobSpec.Build.ImageTag))
		args = append(args, fmt.Sprintf("%s=%s", defaults.RadixBranchEnvironmentVariable, jobSpec.Build.Branch))
		args = append(args, fmt.Sprintf("%s=%s", defaults.RadixCommitIdEnvironmentVariable, jobSpec.Build.CommitID))
		args = append(args, fmt.Sprintf("%s=%s", defaults.RadixPushImageEnvironmentVariable, getPushImageTag(jobSpec.Build.PushImage)))
		args = append(args, fmt.Sprintf("%s=%s", defaults.RadixUseCacheEnvironmentVariable, useImageBuilderCache))
		args = append(args, fmt.Sprintf("%s=%s", defaults.RadixConfigFileEnvironmentVariable, radixConfigFullName))
	case v1.Promote:
		args = append(args, fmt.Sprintf("%s=%s", defaults.RadixPromoteDeploymentEnvironmentVariable, jobSpec.Promote.DeploymentName))
		args = append(args, fmt.Sprintf("%s=%s", defaults.RadixPromoteFromEnvironmentEnvironmentVariable, jobSpec.Promote.FromEnvironment))
		args = append(args, fmt.Sprintf("%s=%s", defaults.RadixPromoteToEnvironmentEnvironmentVariable, jobSpec.Promote.ToEnvironment))
		args = append(args, fmt.Sprintf("%s=%s", defaults.RadixConfigFileEnvironmentVariable, radixConfigFullName))
	case v1.Deploy:
		args = append(args, fmt.Sprintf("%s=%s", defaults.RadixPromoteToEnvironmentEnvironmentVariable, jobSpec.Deploy.ToEnvironment))
		args = append(args, fmt.Sprintf("%s=%s", defaults.RadixConfigFileEnvironmentVariable, radixConfigFullName))
	}

	return args, nil
}

func getPipelineJobLabels(appName, jobName string, jobSpec v1.RadixJobSpec, pipeline *pipelineJob.Definition) map[string]string {
	// Base labels for all types of pipeline
	labels := map[string]string{
		kube.RadixJobNameLabel: jobName,
		kube.RadixJobTypeLabel: kube.RadixJobTypeJob,
		"radix-pipeline":       string(pipeline.Type),
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

func (job *Job) getJobConditionFromJobStatus(jobStatus batchv1.JobStatus) (v1.RadixJobCondition, error) {
	if jobStatus.Failed > 0 {
		return v1.JobFailed, nil
	}
	if jobStatus.Active > 0 {
		return v1.JobRunning, nil

	}
	if jobStatus.Succeeded > 0 {
		jobResult, err := job.getRadixJobResult()
		if err != nil {
			return v1.JobSucceeded, err
		}
		if jobResult.Result == v1.RadixJobResultStoppedNoChanges || job.radixJob.Status.Condition == v1.JobStoppedNoChanges {
			return v1.JobStoppedNoChanges, nil
		}
		return v1.JobSucceeded, nil
	}
	return v1.JobWaiting, nil
}

func (job *Job) getRadixJobResult() (*v1.RadixJobResult, error) {
	jobNamespace := job.radixJob.GetNamespace()
	jobName := job.radixJob.GetName()
	radixJobNameReq := getRadixJobNameSelectorRequirement(jobName)
	pipelineResultConfigMapReq := getPipelineResultConfigMapSelectorRequirement()
	resultConfigMaps, err := job.kubeutil.GetConfigMapListForLabels(jobNamespace, *radixJobNameReq, *pipelineResultConfigMapReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get ConfigMaps while garbage collecting config-maps in %s. Error: %w", jobNamespace, err)
	}
	jobResult := &v1.RadixJobResult{}
	if len(resultConfigMaps) == 0 {
		return jobResult, nil
	}
	if len(resultConfigMaps) > 1 {
		return nil, fmt.Errorf("unexpected multiple Radix pipeline result ConfigMaps for the job %s in %s", jobName, job.radixJob.GetNamespace())
	}
	if resultContent, ok := resultConfigMaps[0].Data[ResultContent]; ok && len(resultContent) > 0 {
		err = yaml.Unmarshal([]byte(resultContent), jobResult)
		if err != nil {
			return nil, err
		}
	}
	return jobResult, nil
}

func getPipelineResultConfigMapSelectorRequirement() *labels.Requirement {
	requirement, _ := labels.NewRequirement(kube.RadixConfigMapTypeLabel, selection.Equals, []string{string(kube.RadixPipelineResultConfigMap)})
	return requirement
}

func getRadixJobNameSelectorRequirement(jobName string) *labels.Requirement {
	requirement, _ := labels.NewRequirement(kube.RadixJobNameLabel, selection.Equals, []string{jobName})
	return requirement
}

func getRadixJobNameExistsSelectorRequirement() *labels.Requirement {
	requirement, _ := labels.NewRequirement(kube.RadixJobNameLabel, selection.Exists, []string{})
	return requirement
}
