package job

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/equinor/radix-common/utils/maps"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/git"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	pipelineJob "github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/rs/zerolog/log"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/yaml"
)

const (
	workerImage = "radix-pipeline"
	// ResultContent of the pipeline job, passed via ConfigMap as v1.RadixJobResult structure
	ResultContent = "ResultContent"
	runAsUser     = 1000
	runAsGroup    = 1000
	fsGroup       = 1000
)

func (job *Job) createPipelineJob(ctx context.Context) error {
	namespace := job.radixJob.Namespace

	jobConfig, err := job.getPipelineJobConfig(ctx)
	if err != nil {
		return err
	}

	_, err = job.kubeclient.BatchV1().Jobs(namespace).Create(ctx, jobConfig, metav1.CreateOptions{})
	return err
}

func (job *Job) getPipelineJobConfig(ctx context.Context) (*batchv1.Job, error) {
	radixRegistration, err := job.radixclient.RadixV1().RadixRegistrations().Get(ctx, job.radixJob.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	radixConfigFullName, err := getRadixConfigFullName(radixRegistration)
	if err != nil {
		return nil, err
	}

	containerRegistry, err := defaults.GetEnvVar(defaults.ContainerRegistryEnvironmentVariable)
	if err != nil {
		return nil, err
	}
	imageTag := fmt.Sprintf("%s/%s:%s", containerRegistry, workerImage, job.config.PipelineJobConfig.PipelineImageTag)
	log.Ctx(ctx).Info().Msgf("Using image: %s", imageTag)

	backOffLimit := int32(0)
	appName := job.radixJob.Spec.AppName
	jobName := job.radixJob.Name

	pipeline, err := pipelineJob.GetPipelineFromName(string(job.radixJob.Spec.PipeLineType))
	if err != nil {
		return nil, err
	}

	workspace := git.Workspace
	containerArguments, err := job.getPipelineJobArguments(ctx, appName, jobName, workspace, radixConfigFullName, job.radixJob.Spec, pipeline)
	if err != nil {
		return nil, err
	}

	initContainers := job.getInitContainersForRadixConfig(radixRegistration, workspace)

	jobCfg := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   jobName,
			Labels: getPipelineJobLabels(appName, jobName, job.radixJob.Spec, pipeline),
			Annotations: map[string]string{
				kube.RadixBranchAnnotation:     job.radixJob.Spec.Build.Branch, //nolint:staticcheck
				kube.RadixGitRefAnnotation:     job.radixJob.Spec.Build.GitRef,
				kube.RadixGitRefTypeAnnotation: string(job.radixJob.Spec.Build.GitRefType),
			},
			OwnerReferences: GetOwnerReference(job.radixJob),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backOffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      getPipelineJobPodLabels(jobName, job.registration.Spec.AppID),
					Annotations: annotations.ForClusterAutoscalerSafeToEvict(false),
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: defaults.PipelineServiceAccountName,
					SecurityContext: securitycontext.Pod(
						securitycontext.WithPodFSGroup(fsGroup),
						securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault)),
					InitContainers: initContainers,
					Containers: []corev1.Container{
						{
							Name:            defaults.RadixPipelineJobPipelineContainerName,
							Image:           imageTag,
							ImagePullPolicy: corev1.PullAlways,
							VolumeMounts:    git.GetJobContainerVolumeMounts(workspace),
							Args:            containerArguments,
							SecurityContext: securitycontext.Container(
								securitycontext.WithContainerDropAllCapabilities(),
								securitycontext.WithContainerSeccompProfileType(corev1.SeccompProfileTypeRuntimeDefault),
								securitycontext.WithContainerRunAsGroup(runAsGroup),
								securitycontext.WithContainerRunAsUser(runAsUser),
								securitycontext.WithReadOnlyRootFileSystem(pointers.Ptr(true))),
							Resources: getPipelineRunnerResources(),
						},
					},
					Volumes:       git.GetJobVolumes(),
					RestartPolicy: "Never",
					Affinity:      utils.GetAffinityForPipelineJob(string(radixv1.RuntimeArchitectureArm64)),
					Tolerations:   utils.GetPipelineJobPodSpecTolerations(),
				},
			},
		},
	}

	return &jobCfg, nil
}

func getPipelineRunnerResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("1000Mi"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("250Mi"),
		},
	}
}

func getRadixConfigFullName(radixRegistration *radixv1.RadixRegistration) (string, error) {
	radixConfigFullName := radixRegistration.Spec.RadixConfigFullName
	if len(radixConfigFullName) == 0 {
		radixConfigFullName = defaults.DefaultRadixConfigFileName
	}
	if err := radixvalidators.ValidateRadixConfigFullName(radixConfigFullName); err != nil {
		return "", err
	}
	return radixConfigFullName, nil
}

func (job *Job) getInitContainersForRadixConfig(rr *radixv1.RadixRegistration, workspace string) []corev1.Container {
	return git.CloneInitContainersWithContainerName(rr.Spec.CloneURL, rr.Spec.ConfigBranch, "", workspace, false, false, git.CloneConfigContainerName, *job.config.PipelineJobConfig.GitCloneConfig)
}

func (job *Job) getPipelineJobArguments(ctx context.Context, appName, jobName, workspace, radixConfigFullName string, jobSpec radixv1.RadixJobSpec, pipeline *pipelineJob.Definition) ([]string, error) {
	clusterType := os.Getenv(defaults.OperatorClusterTypeEnvironmentVariable)
	radixZone := os.Getenv(defaults.RadixZoneEnvironmentVariable)

	clusterName, err := job.kubeutil.GetClusterName(ctx)
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
	subscriptionId, err := job.kubeutil.GetSubscriptionId(ctx)
	if err != nil {
		return nil, err
	}

	if job.config.PipelineJobConfig.AppBuilderResourcesRequestsMemory == nil || job.config.PipelineJobConfig.AppBuilderResourcesRequestsMemory.IsZero() ||
		job.config.PipelineJobConfig.AppBuilderResourcesRequestsCPU == nil || job.config.PipelineJobConfig.AppBuilderResourcesRequestsCPU.IsZero() ||
		job.config.PipelineJobConfig.AppBuilderResourcesLimitsMemory == nil || job.config.PipelineJobConfig.AppBuilderResourcesLimitsMemory.IsZero() ||
		job.config.PipelineJobConfig.AppBuilderResourcesLimitsCPU == nil || job.config.PipelineJobConfig.AppBuilderResourcesLimitsCPU.IsZero() {
		return nil, fmt.Errorf("invalid or missing app builder resources")
	}

	// Base arguments for all types of pipeline
	args := []string{
		fmt.Sprintf("--%s=%s", defaults.RadixAppEnvironmentVariable, appName),
		fmt.Sprintf("--%s=%s", defaults.RadixPipelineJobEnvironmentVariable, jobName),
		fmt.Sprintf("--%s=%s", defaults.RadixPipelineTypeEnvironmentVariable, pipeline.Type),
		fmt.Sprintf("--%s=%s", defaults.OperatorAppBuilderResourcesRequestsMemoryEnvironmentVariable, job.config.PipelineJobConfig.AppBuilderResourcesRequestsMemory.String()),
		fmt.Sprintf("--%s=%s", defaults.OperatorAppBuilderResourcesRequestsCPUEnvironmentVariable, job.config.PipelineJobConfig.AppBuilderResourcesRequestsCPU.String()),
		fmt.Sprintf("--%s=%s", defaults.OperatorAppBuilderResourcesLimitsMemoryEnvironmentVariable, job.config.PipelineJobConfig.AppBuilderResourcesLimitsMemory.String()),
		fmt.Sprintf("--%s=%s", defaults.OperatorAppBuilderResourcesLimitsCPUEnvironmentVariable, job.config.PipelineJobConfig.AppBuilderResourcesLimitsCPU.String()),
		fmt.Sprintf("--%s=%s", defaults.RadixExternalRegistryDefaultAuthEnvironmentVariable, job.config.ContainerRegistryConfig.ExternalRegistryAuthSecret),

		// Pass tekton and builder images
		fmt.Sprintf("--%s=%s", defaults.RadixImageBuilderEnvironmentVariable, os.Getenv(defaults.RadixImageBuilderEnvironmentVariable)),
		fmt.Sprintf("--%s=%s", defaults.RadixBuildKitImageBuilderEnvironmentVariable, os.Getenv(defaults.RadixBuildKitImageBuilderEnvironmentVariable)),
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
		fmt.Sprintf("--%s=%s", defaults.RadixGithubWorkspaceEnvironmentVariable, workspace),
		fmt.Sprintf("--%s=%s", defaults.RadixConfigFileEnvironmentVariable, radixConfigFullName),
		fmt.Sprintf("--%s=%v", defaults.RadixPipelineJobTriggeredFromWebhookEnvironmentVariable, job.radixJob.Spec.TriggeredFromWebhook),
	}

	// Pass git clone init container images
	args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixGitCloneNsLookupImageEnvironmentVariable, job.config.PipelineJobConfig.GitCloneConfig.NSlookupImage))
	args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixGitCloneGitImageEnvironmentVariable, job.config.PipelineJobConfig.GitCloneConfig.GitImage))
	args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixGitCloneBashImageEnvironmentVariable, job.config.PipelineJobConfig.GitCloneConfig.BashImage))

	switch pipeline.Type {
	case radixv1.BuildDeploy, radixv1.Build:
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixImageTagEnvironmentVariable, jobSpec.Build.ImageTag))
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixBranchEnvironmentVariable, jobSpec.Build.Branch)) //nolint:staticcheck
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixGitRefEnvironmentVariable, jobSpec.Build.GitRef))
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixGitRefTypeEnvironmentVariable, jobSpec.Build.GitRefType))
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixPipelineJobToEnvironmentEnvironmentVariable, jobSpec.Build.ToEnvironment))
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixCommitIdEnvironmentVariable, jobSpec.Build.CommitID))
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixPushImageEnvironmentVariable, getPushImageTag(jobSpec.Build.PushImage)))
		if jobSpec.Build.OverrideUseBuildCache != nil {
			args = append(args, fmt.Sprintf("--%s=%v", defaults.RadixOverrideUseBuildCacheEnvironmentVariable, *jobSpec.Build.OverrideUseBuildCache))
		}
		if jobSpec.Build.RefreshBuildCache != nil {
			args = append(args, fmt.Sprintf("--%s=%v", defaults.RadixRefreshBuildCacheEnvironmentVariable, *jobSpec.Build.RefreshBuildCache))
		}
	case radixv1.Promote:
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixPromoteDeploymentEnvironmentVariable, jobSpec.Promote.DeploymentName))
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixPromoteFromEnvironmentEnvironmentVariable, jobSpec.Promote.FromEnvironment))
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixPipelineJobToEnvironmentEnvironmentVariable, jobSpec.Promote.ToEnvironment))
	case radixv1.Deploy:
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixPipelineJobToEnvironmentEnvironmentVariable, jobSpec.Deploy.ToEnvironment))
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixCommitIdEnvironmentVariable, jobSpec.Deploy.CommitID))
		for componentName, imageTagName := range jobSpec.Deploy.ImageTagNames {
			args = append(args, fmt.Sprintf("--%s=%s=%s", defaults.RadixImageTagNameEnvironmentVariable, componentName, imageTagName))
		}
		args = append(args, fmt.Sprintf("--%s=%s", defaults.RadixComponentsToDeployVariable, strings.Join(jobSpec.Deploy.ComponentsToDeploy, ",")))
	case radixv1.ApplyConfig:
		args = append(args, fmt.Sprintf("--%s=%v", defaults.RadixPipelineApplyConfigDeployExternalDNSFlag, jobSpec.ApplyConfig.DeployExternalDNS))
	}

	return args, nil
}

func getPipelineJobLabels(appName, jobName string, jobSpec radixv1.RadixJobSpec, pipeline *pipelineJob.Definition) map[string]string {
	// Base labels for all types of pipeline
	labels := radixlabels.Merge(
		radixlabels.ForApplicationName(appName),
		radixlabels.ForPipelineJobName(jobName),
		radixlabels.ForPipelineJobType(),
		radixlabels.ForPipelineJobPipelineType(pipeline.Type),
	)

	switch pipeline.Type {
	case radixv1.BuildDeploy, radixv1.Build:
		labels = radixlabels.Merge(
			labels,
			radixlabels.ForCommitId(jobSpec.Build.CommitID),
			radixlabels.ForRadixImageTag(jobSpec.Build.ImageTag),
		)
	}

	return labels
}

func getPipelineJobPodLabels(jobName string, appID radixv1.ULID) map[string]string {
	return radixlabels.Merge(
		radixlabels.ForApplicationID(appID),
		radixlabels.ForPipelineJobName(jobName),
	)
}

func getPushImageTag(pushImage bool) string {
	if pushImage {
		return "1"
	}

	return "0"
}

func (job *Job) getJobConditionFromJobStatus(ctx context.Context, jobStatus batchv1.JobStatus) (radixv1.RadixJobCondition, error) {
	if jobStatus.Failed > 0 {
		return radixv1.JobFailed, nil
	}
	if jobStatus.Active > 0 {
		return radixv1.JobRunning, nil

	}
	if jobStatus.Succeeded > 0 {
		jobResult, err := job.getRadixJobResult(ctx)
		if err != nil {
			return radixv1.JobSucceeded, err
		}
		if jobResult.Result == radixv1.RadixJobResultStoppedNoChanges || job.radixJob.Status.Condition == radixv1.JobStoppedNoChanges {
			return radixv1.JobStoppedNoChanges, nil
		}
		return radixv1.JobSucceeded, nil
	}
	return radixv1.JobWaiting, nil
}

func (job *Job) getRadixJobResult(ctx context.Context) (*radixv1.RadixJobResult, error) {
	namespace := job.radixJob.GetNamespace()
	jobName := job.radixJob.GetName()
	configMaps, err := job.kubeutil.ListConfigMapsWithSelector(ctx, namespace, getRadixPipelineJobResultConfigMapSelector(jobName))
	if err != nil {
		return nil, fmt.Errorf("failed to get ConfigMaps while garbage collecting config-maps in %s. Error: %w", namespace, err)
	}
	if len(configMaps) > 1 {
		return nil, fmt.Errorf("unexpected multiple Radix pipeline result ConfigMaps for the job %s in %s", jobName, job.radixJob.GetNamespace())
	}
	radixJobResult := &radixv1.RadixJobResult{}
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
