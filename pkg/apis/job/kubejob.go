package job

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/equinor/radix-common/utils/maps"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	pipelineJob "github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/yaml"
)

const (
	tektonImage = "radix-tekton"
	workerImage = "radix-pipeline"
	// ResultContent of the pipeline job, passed via ConfigMap as v1.RadixJobResult structure
	ResultContent = "ResultContent"
	runAsUser     = 1000
	runAsGroup    = 1000
	fsGroup       = 1000
)

func (job *Job) createPipelineJob() error {
	namespace := job.radixJob.Namespace

	jobConfig, err := job.getPipelineJobConfig()
	if err != nil {
		return err
	}

	_, err = job.kubeclient.BatchV1().Jobs(namespace).Create(context.TODO(), jobConfig, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (job *Job) getPipelineJobConfig() (*batchv1.Job, error) {
	containerRegistry, err := defaults.GetEnvVar(defaults.ContainerRegistryEnvironmentVariable)
	if err != nil {
		return nil, err
	}
	imageTag := fmt.Sprintf("%s/%s:%s", containerRegistry, workerImage, job.radixJob.Spec.PipelineImage)
	job.logger.Info().Msgf("Using image: %s", imageTag)

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
			OwnerReferences: GetOwnerReference(job.radixJob),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backOffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      radixlabels.ForPipelineJobName(jobName),
					Annotations: annotations.ForClusterAutoscalerSafeToEvict(false),
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: defaults.PipelineServiceAccountName,
					SecurityContext: securitycontext.Pod(
						securitycontext.WithPodFSGroup(fsGroup),
						securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault)),
					Containers: []corev1.Container{
						{
							Name:            defaults.RadixPipelineJobPipelineContainerName,
							Image:           imageTag,
							ImagePullPolicy: corev1.PullAlways,
							Args:            containerArguments,
							SecurityContext: securitycontext.Container(
								securitycontext.WithContainerDropAllCapabilities(),
								securitycontext.WithContainerSeccompProfileType(corev1.SeccompProfileTypeRuntimeDefault),
								securitycontext.WithContainerRunAsGroup(runAsGroup),
								securitycontext.WithContainerRunAsUser(runAsUser)),
						},
					},
					RestartPolicy: "Never",
					Affinity:      utils.GetPipelineJobPodSpecAffinity(),
					Tolerations:   utils.GetPipelineJobPodSpecTolerations(),
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
	containerRegistry, err := defaults.GetEnvVar(defaults.ContainerRegistryEnvironmentVariable)
	if err != nil {
		return nil, err
	}
	appContainerRegistry, err := defaults.GetEnvVar(defaults.AppContainerRegistryEnvironmentVariable)
	if err != nil {
		return nil, err
	}
	subscriptionId, err := job.kubeutil.GetSubscriptionId()
	if err != nil {
		return nil, err
	}

	if job.config.PipelineJobConfig.AppBuilderResourcesRequestsMemory == nil || job.config.PipelineJobConfig.AppBuilderResourcesRequestsMemory.IsZero() ||
		job.config.PipelineJobConfig.AppBuilderResourcesRequestsCPU == nil || job.config.PipelineJobConfig.AppBuilderResourcesRequestsCPU.IsZero() ||
		job.config.PipelineJobConfig.AppBuilderResourcesLimitsMemory == nil || job.config.PipelineJobConfig.AppBuilderResourcesLimitsMemory.IsZero() {
		return nil, fmt.Errorf("invalid or missing app builder resources")
	}

	// TODO: Remove fallback to Operator GetEnv when Radix-API is upgrade
	radixTektonImage := os.Getenv(defaults.RadixTektonPipelineImageEnvironmentVariable)
	if job.radixJob.Spec.TektonImage != "" {
		radixTektonImage = fmt.Sprintf("%s:%s", tektonImage, job.radixJob.Spec.TektonImage)
	}

	// Base arguments for all types of pipeline
	args := []string{
		fmt.Sprintf("--%s=%s", defaults.RadixAppEnvironmentVariable, appName),
		fmt.Sprintf("--%s=%s", defaults.RadixPipelineJobEnvironmentVariable, jobName),
		fmt.Sprintf("--%s=%s", defaults.RadixPipelineTypeEnvironmentVariable, pipeline.Type),
		fmt.Sprintf("--%s=%s", defaults.OperatorAppBuilderResourcesRequestsMemoryEnvironmentVariable, job.config.PipelineJobConfig.AppBuilderResourcesRequestsMemory.String()),
		fmt.Sprintf("--%s=%s", defaults.OperatorAppBuilderResourcesRequestsCPUEnvironmentVariable, job.config.PipelineJobConfig.AppBuilderResourcesRequestsCPU.String()),
		fmt.Sprintf("--%s=%s", defaults.OperatorAppBuilderResourcesLimitsMemoryEnvironmentVariable, job.config.PipelineJobConfig.AppBuilderResourcesLimitsMemory.String()),

		// Pass tekton and builder images
		fmt.Sprintf("--%s=%s", defaults.RadixTektonPipelineImageEnvironmentVariable, radixTektonImage),
		fmt.Sprintf("--%s=%s", defaults.RadixImageBuilderEnvironmentVariable, os.Getenv(defaults.RadixImageBuilderEnvironmentVariable)),
		fmt.Sprintf("--%s=%s", defaults.RadixBuildahImageBuilderEnvironmentVariable, os.Getenv(defaults.RadixBuildahImageBuilderEnvironmentVariable)),
		fmt.Sprintf("--%s=%s", defaults.SeccompProfileFileNameEnvironmentVariable, os.Getenv(defaults.SeccompProfileFileNameEnvironmentVariable)),

		// Used for tagging source of image
		fmt.Sprintf("--%s=%s", defaults.RadixClusterTypeEnvironmentVariable, clusterType),
		fmt.Sprintf("--%s=%s", defaults.RadixZoneEnvironmentVariable, radixZone),
		fmt.Sprintf("--%s=%s", defaults.ClusternameEnvironmentVariable, clusterName),
		fmt.Sprintf("--%s=%s", defaults.ContainerRegistryEnvironmentVariable, containerRegistry),
		fmt.Sprintf("--%s=%s", defaults.AppContainerRegistryEnvironmentVariable, appContainerRegistry),
		fmt.Sprintf("--%s=%s", defaults.AzureSubscriptionIdEnvironmentVariable, subscriptionId),
		fmt.Sprintf("--%s=%s", defaults.RadixReservedAppDNSAliasesEnvironmentVariable, maps.ToString(job.config.DNSConfig.ReservedAppDNSAliases)),
		fmt.Sprintf("--%s=%s", defaults.RadixReservedDNSAliasesEnvironmentVariable, strings.Join(job.config.DNSConfig.ReservedDNSAliases, ",")),
	}

	radixConfigFullName := jobSpec.RadixConfigFullName
	if len(radixConfigFullName) == 0 {
		radixConfigFullName = fmt.Sprintf("%s/%s", git.Workspace, defaults.DefaultRadixConfigFileName)
	}
	switch pipeline.Type {
	case v1.BuildDeploy, v1.Build:
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixImageTagEnvironmentVariable, jobSpec.Build.ImageTag))
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixBranchEnvironmentVariable, jobSpec.Build.Branch))
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixCommitIdEnvironmentVariable, jobSpec.Build.CommitID))
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixPushImageEnvironmentVariable, getPushImageTag(jobSpec.Build.PushImage)))
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixUseCacheEnvironmentVariable, useImageBuilderCache))
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixConfigFileEnvironmentVariable, radixConfigFullName))
	case v1.Promote:
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixPromoteDeploymentEnvironmentVariable, jobSpec.Promote.DeploymentName))
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixPromoteFromEnvironmentEnvironmentVariable, jobSpec.Promote.FromEnvironment))
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixPromoteToEnvironmentEnvironmentVariable, jobSpec.Promote.ToEnvironment))
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixConfigFileEnvironmentVariable, radixConfigFullName))
	case v1.Deploy:
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixPromoteToEnvironmentEnvironmentVariable, jobSpec.Deploy.ToEnvironment))
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixCommitIdEnvironmentVariable, jobSpec.Deploy.CommitID))
		for componentName, imageTagName := range jobSpec.Deploy.ImageTagNames {
			args = append(args, fmt.Sprintf("--%s=%s=%s", defaults.RadixImageTagNameEnvironmentVariable, componentName, imageTagName))
		}
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixConfigFileEnvironmentVariable, radixConfigFullName))
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixComponentsToDeployVariable, strings.Join(jobSpec.Deploy.ComponentsToDeploy, ",")))
	}

	return args, nil
}

func getPipelineJobLabels(appName, jobName string, jobSpec v1.RadixJobSpec, pipeline *pipelineJob.Definition) map[string]string {
	// Base labels for all types of pipeline
	labels := radixlabels.Merge(
		radixlabels.ForApplicationName(appName),
		radixlabels.ForPipelineJobName(jobName),
		radixlabels.ForPipelineJobType(),
		radixlabels.ForPipelineJobPipelineType(pipeline.Type),
	)

	switch pipeline.Type {
	case v1.BuildDeploy, v1.Build:
		labels = radixlabels.Merge(
			labels,
			radixlabels.ForCommitId(jobSpec.Build.CommitID),
			radixlabels.ForRadixImageTag(jobSpec.Build.ImageTag),
		)
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
	namespace := job.radixJob.GetNamespace()
	jobName := job.radixJob.GetName()
	configMaps, err := job.kubeutil.ListConfigMapsWithSelector(namespace, getRadixPipelineJobResultConfigMapSelector(jobName))
	if err != nil {
		return nil, fmt.Errorf("failed to get ConfigMaps while garbage collecting config-maps in %s. Error: %w", namespace, err)
	}
	if len(configMaps) > 1 {
		return nil, fmt.Errorf("unexpected multiple Radix pipeline result ConfigMaps for the job %s in %s", jobName, job.radixJob.GetNamespace())
	}
	radixJobResult := &v1.RadixJobResult{}
	if len(configMaps) == 0 {
		return radixJobResult, nil
	}
	if resultContent, ok := configMaps[0].Data[ResultContent]; ok && len(resultContent) > 0 {
		err = yaml.Unmarshal([]byte(resultContent), radixJobResult)
		if err != nil {
			return nil, err
		}
	}
	return radixJobResult, nil
}

func getRadixPipelineJobResultConfigMapSelector(jobName string) string {
	radixJobNameReq, _ := kubelabels.NewRequirement(kube.RadixJobNameLabel, selection.Equals, []string{jobName})
	pipelineResultConfigMapReq, _ := kubelabels.NewRequirement(kube.RadixConfigMapTypeLabel, selection.Equals, []string{string(kube.RadixPipelineResultConfigMap)})
	return kubelabels.NewSelector().Add(*radixJobNameReq, *pipelineResultConfigMapReq).String()
}
