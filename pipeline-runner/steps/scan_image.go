package steps

import (
	"fmt"
	"strings"
	"time"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	jobUtil "github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	internalContainerPrefix   = "internal-"
	waitForDockerHubToRespond = "n=1;max=10;delay=2;while true; do if [ \"$n\" -lt \"$max\" ]; then nslookup hub.docker.com && break; n=$((n+1)); sleep $(($delay*$n)); else echo \"The command has failed after $n attempts.\"; break; fi done"
)

// ScanImageImplementation Step to scan image for vulnerabilities
type ScanImageImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
}

// NewScanImageStep Constructor
func NewScanImageStep() model.Step {
	return &ScanImageImplementation{
		stepType: pipeline.ScanImageStep,
	}
}

// ImplementationForType Override of default step method
func (cli *ScanImageImplementation) ImplementationForType() pipeline.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *ScanImageImplementation) SucceededMsg() string {
	return fmt.Sprintf("Image scan successful for application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *ScanImageImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Image scan successful for application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *ScanImageImplementation) Run(pipelineInfo *model.PipelineInfo) error {
	if pipelineInfo.ComponentImages == nil || len(pipelineInfo.ComponentImages) == 0 {
		// Do nothing and no error
		log.Infof("No component image present to scan for app %s", cli.GetAppName())
		return nil
	}

	log.Infof("Scanning images for app %s", cli.GetAppName())

	namespace := utils.GetAppNamespace(cli.GetAppName())
	job, err := createScanJob(cli.GetAppName(), pipelineInfo.ComponentImages, pipelineInfo.PipelineArguments)
	if err != nil {
		return err
	}

	ownerReference, err := jobUtil.GetOwnerReferenceOfJob(cli.GetRadixclient(), namespace, pipelineInfo.PipelineArguments.JobName)
	if err != nil {
		return err
	}

	job.OwnerReferences = ownerReference

	log.Infof("Apply job (%s) to scan component images for app %s", job.Name, cli.GetAppName())
	job, err = cli.GetKubeclient().BatchV1().Jobs(namespace).Create(job)
	if err != nil {
		return err
	}

	err = cli.GetKubeutil().WaitForCompletionOf(job)
	log.Errorf("Error scanning image for app %s: %v", cli.GetAppName(), err)

	return nil
}

func createScanJob(appName string, componentImages map[string]model.ComponentImage, pipelineArguments model.PipelineArguments) (*batchv1.Job, error) {
	imageScanContainers := createImageScanContainers(componentImages)
	timestamp := time.Now().Format("20060102150405")

	imageTag := pipelineArguments.ImageTag
	jobName := pipelineArguments.JobName

	backOffLimit := int32(0)

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("radix-scanner-%s-%s", timestamp, imageTag),
			Labels: map[string]string{
				kube.RadixJobNameLabel:  jobName,
				kube.RadixAppLabel:      appName,
				kube.RadixImageTagLabel: imageTag,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backOffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						kube.RadixJobNameLabel: jobName,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: "Never",
					InitContainers: []corev1.Container{
						{
							Name:            fmt.Sprintf("%snslookup", internalContainerPrefix),
							Image:           "alpine",
							Args:            []string{waitForDockerHubToRespond},
							Command:         []string{"/bin/sh", "-c"},
							ImagePullPolicy: "Always",
						}},
					Containers: imageScanContainers,
					Volumes: []corev1.Volume{
						{
							Name: azureServicePrincipleSecretName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: azureServicePrincipleSecretName,
								},
							},
						},
					},
				},
			},
		},
	}
	return &job, nil
}

func createImageScanContainers(componentImages map[string]model.ComponentImage) []corev1.Container {
	distinctImages := make(map[string]struct{})
	containers := []corev1.Container{}
	azureServicePrincipleContext := "/radix-image-scanner/.azure"

	for componentName, componentImage := range componentImages {
		if _, contains := distinctImages[componentImage.ImagePath]; contains {
			// Do not scan same image twice
			continue
		}

		volumeMounts := []corev1.VolumeMount{}
		envVars := []corev1.EnvVar{
			{
				Name:  "IMAGE_PATH",
				Value: componentImage.ImagePath,
			},
		}

		if !strings.EqualFold(componentImage.ContainerRegistry, "") {
			envVars = append(envVars,
				corev1.EnvVar{
					Name:  "AZURE_CREDENTIALS",
					Value: fmt.Sprintf("%s/sp_credentials.json", azureServicePrincipleContext),
				})

			volumeMounts = append(volumeMounts,
				corev1.VolumeMount{
					Name:      azureServicePrincipleSecretName,
					MountPath: azureServicePrincipleContext,
					ReadOnly:  true,
				})
		}

		log.Infof("Scanning image %s for component %s", componentImage.ImageName, componentName)
		container := corev1.Container{
			Name:            fmt.Sprintf("scan-%s", componentName),
			Image:           "radixdev.azurecr.io/radix-image-scanner:RA-1004-ScanImages-latest", // todo - version?
			ImagePullPolicy: corev1.PullAlways,
			Env:             envVars,
			VolumeMounts:    volumeMounts,
		}
		containers = append(containers, container)
		distinctImages[componentImage.ImagePath] = struct{}{}
	}
	return containers
}
