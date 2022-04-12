package steps

//import (
//	"context"
//	"fmt"
//	"strings"
//	"time"
//
//	"github.com/equinor/radix-operator/pipeline-runner/model"
//	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
//	"github.com/equinor/radix-operator/pkg/apis/defaults"
//	jobUtil "github.com/equinor/radix-operator/pkg/apis/job"
//	"github.com/equinor/radix-operator/pkg/apis/kube"
//	"github.com/equinor/radix-operator/pkg/apis/pipeline"
//	"github.com/equinor/radix-operator/pkg/apis/utils"
//	"github.com/equinor/radix-operator/pkg/apis/utils/git"
//	log "github.com/sirupsen/logrus"
//	batchv1 "k8s.io/api/batch/v1"
//	corev1 "k8s.io/api/core/v1"
//	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
//)
//
//const containerName = "config-2-map"
//
//// CopyConfigToMapStepImplementation Step to copy RA from clone to configmap
//type CopyConfigToMapStepImplementation struct {
//	stepType pipeline.StepType
//	model.DefaultStepImplementation
//}
//
//// NewCopyConfigToMapStep Constructor
//func NewCopyConfigToMapStep() model.Step {
//	return &CopyConfigToMapStepImplementation{
//		stepType: pipeline.CopyConfigToMapStep,
//	}
//}
//
//// ImplementationForType Override of default step method
//func (cli *CopyConfigToMapStepImplementation) ImplementationForType() pipeline.StepType {
//	return cli.stepType
//}
//
//// SucceededMsg Override of default step method
//func (cli *CopyConfigToMapStepImplementation) SucceededMsg() string {
//	return fmt.Sprintf("Cloned and copied config for application %s", cli.GetAppName())
//}
//
//// ErrorMsg Override of default step method
//func (cli *CopyConfigToMapStepImplementation) ErrorMsg(err error) string {
//	return fmt.Sprintf("Failed to copy config for application %s. Error: %v", cli.GetAppName(), err)
//}
//
//// Run Override of default step method
//func (cli *CopyConfigToMapStepImplementation) Run(pipelineInfo *model.PipelineInfo) error {
//	namespace := utils.GetAppNamespace(cli.GetAppName())
//	job := cli.getJobConfig(namespace, pipelineInfo.ContainerRegistry, pipelineInfo)
//
//	// When debugging pipeline there will be no RJ
//	if !pipelineInfo.PipelineArguments.Debug {
//		ownerReference, err := jobUtil.GetOwnerReferenceOfJob(cli.GetRadixclient(), namespace, pipelineInfo.PipelineArguments.JobName)
//		if err != nil {
//			return err
//		}
//
//		job.OwnerReferences = ownerReference
//	}
//
//	log.Infof("Apply job (%s) to copy radixconfig to configmap for app %s", job.Name, cli.GetAppName())
//	job, err := cli.GetKubeclient().BatchV1().Jobs(namespace).Create(context.TODO(), job, metav1.CreateOptions{})
//	if err != nil {
//		return err
//	}
//
//	return cli.GetKubeutil().WaitForCompletionOf(job)
//}
//
//func (cli *CopyConfigToMapStepImplementation) getJobConfig(namespace, containerRegistry string, pipelineInfo *model.PipelineInfo) *batchv1.Job {
//	registration := cli.GetRegistration()
//	imageTag := pipelineInfo.PipelineArguments.ImageTag
//	jobName := pipelineInfo.PipelineArguments.JobName
//	timestamp := time.Now().Format("20060102150405")
//	configBranch := applicationconfig.GetConfigBranch(registration)
//	hash := strings.ToLower(utils.RandStringStrSeed(5, pipelineInfo.PipelineArguments.JobName))
//
//	backOffLimit := int32(0)
//	return &batchv1.Job{
//		ObjectMeta: metav1.ObjectMeta{
//			Name: fmt.Sprintf("radix-clone-config-%s-%s-%s", timestamp, imageTag, hash),
//			Labels: map[string]string{
//				kube.RadixJobNameLabel:  jobName,
//				kube.RadixAppLabel:      cli.GetAppName(),
//				kube.RadixImageTagLabel: imageTag,
//				kube.RadixJobTypeLabel:  kube.RadixJobTypeCloneConfig,
//			},
//		},
//		Spec: batchv1.JobSpec{
//			BackoffLimit: &backOffLimit,
//			Template: corev1.PodTemplateSpec{
//				Spec: corev1.PodSpec{
//					ServiceAccountName: defaults.ConfigToMapRunnerRoleName,
//					SecurityContext:    &pipelineInfo.PipelineArguments.PodSecurityContext,
//					InitContainers:     getInitContainers(registration.Spec.CloneURL, configBranch, pipelineInfo.PipelineArguments.ContainerSecurityContext),
//					Containers: []corev1.Container{
//						{
//							Name:            containerName,
//							Image:           fmt.Sprintf("%s/%s", containerRegistry, pipelineInfo.PipelineArguments.ConfigToMap),
//							ImagePullPolicy: corev1.PullAlways,
//							Args: []string{
//								fmt.Sprintf("--namespace=%s", namespace),
//								fmt.Sprintf("--configmap-name=%s", pipelineInfo.RadixConfigMapName),
//								fmt.Sprintf("--file=%s", pipelineInfo.PipelineArguments.RadixConfigFile),
//							},
//							VolumeMounts:    getJobContainerVolumeMounts(),
//							SecurityContext: &pipelineInfo.PipelineArguments.ContainerSecurityContext,
//						},
//					},
//					Volumes:       getJobVolumes(),
//					RestartPolicy: "Never",
//				},
//			},
//		},
//	}
//
//}
//
//func getInitContainers(sshURL string, configBranch string, containerSecContext corev1.SecurityContext) []corev1.Container {
//	return git.CloneInitContainersWithContainerName(sshURL, configBranch, git.CloneConfigContainerName, containerSecContext)
//}
//
//func getJobVolumes() []corev1.Volume {
//	defaultMode := int32(256)
//
//	volumes := []corev1.Volume{
//		{
//			Name: git.BuildContextVolumeName,
//		},
//		{
//			Name: git.GitSSHKeyVolumeName,
//			VolumeSource: corev1.VolumeSource{
//				Secret: &corev1.SecretVolumeSource{
//					SecretName:  git.GitSSHKeyVolumeName,
//					DefaultMode: &defaultMode,
//				},
//			},
//		},
//	}
//
//	return volumes
//}
//
//func getJobContainerVolumeMounts() []corev1.VolumeMount {
//	return []corev1.VolumeMount{
//		{
//			Name:      git.BuildContextVolumeName,
//			MountPath: git.Workspace,
//		},
//	}
//}
