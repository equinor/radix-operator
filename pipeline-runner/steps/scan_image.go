package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	jobUtil "github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	kubeErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
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
	if pipelineInfo.ComponentImages == nil ||
		len(pipelineInfo.ComponentImages) == 0 ||
		noComponentNeedScanning(pipelineInfo.ComponentImages) {
		// Do nothing and no error
		log.Infof("No component image present to scan for app %s", cli.GetAppName())
		return nil
	}

	log.Infof("Scanning images for app %s", cli.GetAppName())

	namespace := utils.GetAppNamespace(cli.GetAppName())
	scannerImage := fmt.Sprintf("%s/%s", pipelineInfo.ContainerRegistry, pipelineInfo.PipelineArguments.ImageScanner)
	job, scanOutputConfigMaps := createScanJob(cli.GetAppName(), scannerImage, pipelineInfo.ComponentImages, pipelineInfo.PipelineArguments)

	if !pipelineInfo.PipelineArguments.Debug {
		ownerReference, err := jobUtil.GetOwnerReferenceOfJob(cli.GetRadixclient(), namespace, pipelineInfo.PipelineArguments.JobName)
		if err != nil {
			return err
		}

		job.OwnerReferences = ownerReference
	}

	log.Infof("Apply job (%s) to scan component images for app %s", job.Name, cli.GetAppName())
	job, err := cli.GetKubeclient().BatchV1().Jobs(namespace).Create(context.TODO(), job, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	err = cli.GetKubeutil().WaitForCompletionOf(job)
	if err != nil {
		log.Errorf("Error scanning image for app %s: %v", cli.GetAppName(), err)
	}

	if err = setOwnerReferenceAndLabelsForScanOutputConfigMaps(cli.GetKubeclient(), scanOutputConfigMaps, namespace, job.OwnerReferences, pipelineInfo.RadixApplication.Name); err != nil {
		return err
	}

	return nil
}

func setOwnerReferenceAndLabelsForScanOutputConfigMaps(kubeClient kubernetes.Interface, scanOutputConfigMap pipeline.ContainerOutputName, namespace string, ownerReference []metav1.OwnerReference, appName string) error {
	if scanOutputConfigMap == nil {
		return nil
	}

	for _, configMapName := range scanOutputConfigMap {
		configMap, err := kubeClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), configMapName, metav1.GetOptions{})
		if err != nil {
			// Do not fail the scan job if scanning container failed to create the configmap with scan results
			if kubeErrors.IsNotFound(err) {
				continue
			}

			return err
		}

		configMap.OwnerReferences = ownerReference

		if configMap.Labels == nil {
			configMap.Labels = make(map[string]string)
		}
		configMap.Labels[kube.RadixAppLabel] = appName

		// Retry configmap update on conflict. If final error is other than NotFound (e.g. permission error) we fail the step
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			_, err := kubeClient.CoreV1().ConfigMaps(namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
			return err
		})
		if err != nil && !kubeErrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func noComponentNeedScanning(componentImages map[string]pipeline.ComponentImage) bool {
	for _, componentImage := range componentImages {
		if componentImage.Scan {
			return false
		}
	}

	return true
}

func createScanJob(appName, scannerImage string, componentImages map[string]pipeline.ComponentImage, pipelineArguments model.PipelineArguments) (*batchv1.Job, pipeline.ContainerOutputName) {
	timestamp := time.Now().Format("20060102150405")
	imageTag := pipelineArguments.ImageTag
	jobName := pipelineArguments.JobName
	hash := strings.ToLower(utils.RandStringStrSeed(5, pipelineArguments.JobName))
	scanJobName := fmt.Sprintf("radix-scanner-%s-%s-%s", timestamp, imageTag, hash)
	backOffLimit := int32(0)

	imageScanContainers, imageScanComponentImages, containerOutput := createImageScanContainers(appName, scannerImage, scanJobName, componentImages, pipelineArguments.ContainerSecurityContext)

	componentImagesAnnotation, _ := json.Marshal(imageScanComponentImages)
	containerOutputAnnotation, _ := json.Marshal(containerOutput)

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: scanJobName,
			Labels: map[string]string{
				kube.RadixJobNameLabel:  jobName,
				kube.RadixAppLabel:      appName,
				kube.RadixImageTagLabel: imageTag,
				kube.RadixJobTypeLabel:  kube.RadixJobTypeScan,
			},
			Annotations: map[string]string{
				kube.RadixComponentImagesAnnotation: string(componentImagesAnnotation),
				kube.RadixContainerOutputAnnotation: string(containerOutputAnnotation),
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
					SecurityContext:    &pipelineArguments.PodSecurityContext,
					ServiceAccountName: defaults.ScanImageRunnerRoleName,
					RestartPolicy:      "Never",
					InitContainers: []corev1.Container{
						{
							Name:            fmt.Sprintf("%snslookup", internalContainerPrefix),
							Image:           "alpine",
							Args:            []string{waitForDockerHubToRespond},
							Command:         []string{"/bin/sh", "-c"},
							ImagePullPolicy: "Always",
							SecurityContext: &pipelineArguments.ContainerSecurityContext,
						}},
					Containers: imageScanContainers,
					Volumes: []corev1.Volume{
						{
							Name: defaults.AzureACRServicePrincipleSecretName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: defaults.AzureACRServicePrincipleSecretName,
								},
							},
						},
					},
				},
			},
		},
	}
	return &job, containerOutput
}

func createImageScanContainers(appName, scannerImage, scanJobName string, componentImages map[string]pipeline.ComponentImage, containerSecContext corev1.SecurityContext) ([]corev1.Container, map[string]pipeline.ComponentImage, pipeline.ContainerOutputName) {
	distinctImages := make(map[string]struct{})
	imageScanComponentImages := make(map[string]pipeline.ComponentImage)

	containers := []corev1.Container{}
	azureServicePrincipleContext := "/radix-image-scanner/.azure"

	scanResultConfigMapNamespace := utils.GetAppNamespace(appName)
	containerOutputName := make(pipeline.ContainerOutputName)

	for componentName, componentImage := range componentImages {
		if !componentImage.Scan {
			log.Debugf("Skip scanning image %s for component %s", componentImage.ImageName, componentName)
			continue
		}

		scanContainerName := fmt.Sprintf("scan-%s", componentImage.ImageName)
		imageScanComponentImages[componentName] = pipeline.ComponentImage{
			ContainerName: scanContainerName,
			ImageName:     componentImage.ImageName,
			ImagePath:     componentImage.ImagePath,
		}

		if _, contains := distinctImages[componentImage.ImagePath]; contains {
			// Do not scan same image twice
			continue
		}

		scanOutputConfigMapName := fmt.Sprintf("%s-%s", scanJobName, scanContainerName)
		containerOutputName[scanContainerName] = scanOutputConfigMapName

		log.Infof("Scanning image %s for component %s", componentImage.ImageName, componentName)
		envVars := []corev1.EnvVar{
			{
				Name:  "IMAGE_PATH",
				Value: componentImage.ImagePath,
			},
			{
				Name:  "AZURE_CREDENTIALS",
				Value: fmt.Sprintf("%s/sp_credentials.json", azureServicePrincipleContext),
			},
			{
				Name:  "OUTPUT_CONFIGMAP_NAMESPACE",
				Value: scanResultConfigMapNamespace,
			},
			{
				Name:  "OUTPUT_CONFIGMAP_NAME",
				Value: scanOutputConfigMapName,
			},
			{
				Name:  "VULNERABILITY_LIST_KEY",
				Value: defaults.RadixPipelineScanStepVulnerabilityListKey,
			},
			{
				Name:  "VULNERABILITY_COUNT_KEY",
				Value: defaults.RadixPipelineScanStepVulnerabilityCountKey,
			},
			{
				Name: "SNYK_TOKEN",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key: "access-token",
						LocalObjectReference: corev1.LocalObjectReference{
							Name: defaults.SnykServiceAccountSecretName,
						},
					},
				},
			},
		}

		volumeMounts := []corev1.VolumeMount{
			{
				Name:      defaults.AzureACRServicePrincipleSecretName,
				MountPath: azureServicePrincipleContext,
				ReadOnly:  true,
			},
		}

		container := corev1.Container{
			Name:            scanContainerName,
			Image:           scannerImage,
			ImagePullPolicy: corev1.PullAlways,
			Env:             envVars,
			VolumeMounts:    volumeMounts,
			SecurityContext: &containerSecContext,
		}
		containers = append(containers, container)
		distinctImages[componentImage.ImagePath] = struct{}{}
	}
	return containers, imageScanComponentImages, containerOutputName
}
