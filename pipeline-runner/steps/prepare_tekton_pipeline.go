package steps

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	jobUtil "github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const containerName = "radix-tekton"

// PrepareTektonPipelineStepImplementation Step to run custom pipeline
type PrepareTektonPipelineStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
}

// NewPrepareTektonPipelineStep Constructor
func NewPrepareTektonPipelineStep() model.Step {
	return &PrepareTektonPipelineStepImplementation{
		stepType: pipeline.PrepareTektonPipelineStep,
	}
}

// ImplementationForType Override of default step method
func (cli *PrepareTektonPipelineStepImplementation) ImplementationForType() pipeline.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *PrepareTektonPipelineStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Succeded: prepare tekton pipeline step for application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *PrepareTektonPipelineStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed prepare tekton pipeline for the application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *PrepareTektonPipelineStepImplementation) Run(pipelineInfo *model.PipelineInfo) error {
	branch := pipelineInfo.PipelineArguments.Branch
	commitID := pipelineInfo.PipelineArguments.CommitID
	appName := cli.GetAppName()
	namespace := utils.GetAppNamespace(appName)
	log.Infof("Run tekton pipeline app %s for branch %s and commit %s", appName, branch, commitID)

	job := cli.getJobConfig(pipelineInfo.ContainerRegistry, pipelineInfo)

	// When debugging pipeline there will be no RJ
	if !pipelineInfo.PipelineArguments.Debug {
		ownerReference, err := jobUtil.GetOwnerReferenceOfJob(cli.GetRadixclient(), namespace, pipelineInfo.PipelineArguments.JobName)
		if err != nil {
			return err
		}

		job.OwnerReferences = ownerReference
	}

	log.Infof("Apply job (%s) to copy radixconfig to configmap for app %s", job.Name, appName)
	job, err := cli.GetKubeclient().BatchV1().Jobs(namespace).Create(context.TODO(), job, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return cli.GetKubeutil().WaitForCompletionOf(job)
}

func (cli *PrepareTektonPipelineStepImplementation) getJobConfig(containerRegistry string, pipelineInfo *model.PipelineInfo) *batchv1.Job {
	registration := cli.GetRegistration()
	imageTag := pipelineInfo.PipelineArguments.ImageTag
	jobName := pipelineInfo.PipelineArguments.JobName
	timestamp := time.Now().Format("20060102150405")
	configBranch := applicationconfig.GetConfigBranch(registration)
	hash := strings.ToLower(utils.RandStringStrSeed(5, pipelineInfo.PipelineArguments.JobName))

	backOffLimit := int32(0)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("radix-tekton-pipeline-%s-%s-%s", timestamp, imageTag, hash),
			Labels: map[string]string{
				kube.RadixJobNameLabel:     jobName,
				kube.RadixAppLabel:         cli.GetAppName(),
				kube.RadixImageTagLabel:    imageTag,
				kube.RadixJobTypeLabel:     kube.RadixJobTypeTektonPipeline,
				kube.RadixPipelineRunLabel: pipelineInfo.RadixPipelineRun,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backOffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: defaults.ConfigToMapRunnerRoleName,
					SecurityContext:    &pipelineInfo.PipelineArguments.PodSecurityContext,
					InitContainers:     getInitContainers(registration.Spec.CloneURL, configBranch, pipelineInfo.PipelineArguments.ContainerSecurityContext),
					Containers: []corev1.Container{
						{
							Name:            containerName,
							Image:           fmt.Sprintf("%s/%s", containerRegistry, pipelineInfo.PipelineArguments.TektonPipeline),
							ImagePullPolicy: corev1.PullAlways,
							VolumeMounts:    getJobContainerVolumeMounts(),
							SecurityContext: &pipelineInfo.PipelineArguments.ContainerSecurityContext,
							Env: []corev1.EnvVar{
								{
									Name: defaults.RadixPodNamespaceEnvironmentVariable,
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
									},
								},
								{
									Name:  defaults.RadixConfigConfigMapNameEnvironmentVariable,
									Value: pipelineInfo.RadixConfigMapName,
								},
								{
									Name:  defaults.RadixConfigFileNameEnvironmentVariable,
									Value: pipelineInfo.PipelineArguments.RadixConfigFile,
								},
								{
									Name:  defaults.RadixBranchEnvironmentVariable,
									Value: pipelineInfo.PipelineArguments.Branch,
								},
								{
									Name:  defaults.RadixPipelineTypeEnvironmentVariable,
									Value: pipelineInfo.PipelineArguments.PipelineType,
								},
								{
									Name:  defaults.RadixImageTagEnvironmentVariable,
									Value: pipelineInfo.PipelineArguments.ImageTag,
								},
							},
						},
					},
					Volumes:       getJobVolumes(),
					RestartPolicy: "Never",
				},
			},
		},
	}

}

func getInitContainers(sshURL string, configBranch string, containerSecContext corev1.SecurityContext) []corev1.Container {
	return git.CloneInitContainersWithContainerName(sshURL, configBranch, git.CloneConfigContainerName, containerSecContext)
}

func getJobVolumes() []corev1.Volume {
	defaultMode := int32(256)

	volumes := []corev1.Volume{
		{
			Name: git.BuildContextVolumeName,
		},
		{
			Name: git.GitSSHKeyVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  git.GitSSHKeyVolumeName,
					DefaultMode: &defaultMode,
				},
			},
		},
	}

	return volumes
}

func getJobContainerVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      git.BuildContextVolumeName,
			MountPath: git.Workspace,
		},
	}
}
