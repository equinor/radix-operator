package steps

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	azureServicePrincipleSecretName = "radix-sp-acr-azure"
	multiComponentImageName         = "multi-component"
)

type componentType struct {
	name           string
	context        string
	dockerFileName string
}

func createACRBuildJob(rr *v1.RadixRegistration, ra *v1.RadixApplication, containerRegistry string, pipelineInfo *model.PipelineInfo, buildSecrets []corev1.EnvVar) (*batchv1.Job, error) {
	appName := rr.Name
	branch := pipelineInfo.PipelineArguments.Branch
	imageTag := pipelineInfo.PipelineArguments.ImageTag
	jobName := pipelineInfo.PipelineArguments.JobName

	initContainers := git.CloneInitContainers(rr.Spec.CloneURL, branch)
	buildContainers := createACRBuildContainers(containerRegistry, appName, pipelineInfo, ra.Spec.Components, buildSecrets)
	timestamp := time.Now().Format("20060102150405")
	defaultMode, backOffLimit := int32(256), int32(0)

	componentImagesAnnotation, _ := json.Marshal(pipelineInfo.ComponentImages)

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("radix-builder-%s-%s", timestamp, imageTag),
			Labels: map[string]string{
				kube.RadixJobNameLabel:  jobName,
				kube.RadixBuildLabel:    fmt.Sprintf("%s-%s", appName, imageTag),
				"radix-app-name":        appName, // For backwards compatibility. Remove when cluster is migrated
				kube.RadixAppLabel:      appName,
				kube.RadixImageTagLabel: imageTag,
				kube.RadixJobTypeLabel:  kube.RadixJobTypeBuild,
			},
			Annotations: map[string]string{
				kube.RadixBranchAnnotation:          branch,
				kube.RadixComponentImagesAnnotation: string(componentImagesAnnotation),
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
					RestartPolicy:  "Never",
					InitContainers: initContainers,
					Containers:     buildContainers,
					Volumes: []corev1.Volume{
						{
							Name: git.BuildContextVolumeName,
						},
						corev1.Volume{
							Name: git.GitSSHKeyVolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  git.GitSSHKeyVolumeName,
									DefaultMode: &defaultMode,
								},
							},
						},
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

func createACRBuildContainers(containerRegistry, appName string, pipelineInfo *model.PipelineInfo, components []v1.RadixComponent, buildSecrets []corev1.EnvVar) []corev1.Container {
	imageTag := pipelineInfo.PipelineArguments.ImageTag
	pushImage := pipelineInfo.PipelineArguments.PushImage

	imageBuilder := pipelineInfo.PipelineArguments.ImageBuilder
	clustertype := pipelineInfo.PipelineArguments.Clustertype
	clustername := pipelineInfo.PipelineArguments.Clustername

	containers := []corev1.Container{}
	azureServicePrincipleContext := "/radix-image-builder/.azure"
	firstPartContainerRegistry := strings.Split(containerRegistry, ".")[0]
	noPushFlag := "--no-push"
	if pushImage {
		noPushFlag = ""
	}

	componentImages := make(map[string]pipeline.ComponentImage)

	// Gather pre-built or public images
	for _, c := range components {
		if c.Image != "" {
			componentImages[c.Name] = pipeline.ComponentImage{Scan: false, ImageName: c.Image, ImagePath: c.Image}
		}
	}

	buildContextComponents := getBuildContextComponents(components)
	numMultiComponentContainers := 0

	for _, components := range buildContextComponents {
		var imageName string

		if len(components) > 1 {
			log.Infof("Multiple components points to the same build context")
			imageName = multiComponentImageName

			if numMultiComponentContainers > 0 {
				// Start indexing them
				imageName = fmt.Sprintf("%s-%d", imageName, numMultiComponentContainers)
			}

			numMultiComponentContainers++
		} else {
			imageName = components[0].name
		}

		buildContainerName := fmt.Sprintf("build-%s", imageName)

		// A multi-component share context and dockerfile
		context := components[0].context
		dockerFile := components[0].dockerFileName

		// Set image back to component(s)
		for _, c := range components {
			componentImages[c.name] = pipeline.ComponentImage{
				ContainerName: buildContainerName,
				ImageName:     imageName,
				ImagePath:     utils.GetImagePath(containerRegistry, appName, imageName, imageTag),
				Scan:          true,
			}
		}

		imagePath := utils.GetImagePath(containerRegistry, appName, imageName, imageTag)

		// For extra meta inforamtion about an image
		clustertypeImage := utils.GetImagePath(containerRegistry, appName, imageName, fmt.Sprintf("%s-%s", clustertype, imageTag))
		clusternameImage := utils.GetImagePath(containerRegistry, appName, imageName, fmt.Sprintf("%s-%s", clustername, imageTag))

		envVars := []corev1.EnvVar{
			{
				Name:  "DOCKER_FILE_NAME",
				Value: dockerFile,
			},
			{
				Name:  "DOCKER_REGISTRY",
				Value: firstPartContainerRegistry,
			},
			{
				Name:  "IMAGE",
				Value: imagePath,
			},
			{
				Name:  "CONTEXT",
				Value: context,
			},
			{
				Name:  "NO_PUSH",
				Value: noPushFlag,
			},
			{
				Name:  "AZURE_CREDENTIALS",
				Value: fmt.Sprintf("%s/sp_credentials.json", azureServicePrincipleContext),
			},

			// Extra meta information
			{
				Name:  "CLUSTERTYPE_IMAGE",
				Value: clustertypeImage,
			},
			{
				Name:  "CLUSTERNAME_IMAGE",
				Value: clusternameImage,
			},
		}

		envVars = append(envVars, buildSecrets...)
		imageBuilder := fmt.Sprintf("%s/%s", containerRegistry, imageBuilder)

		container := corev1.Container{
			Name:            buildContainerName,
			Image:           imageBuilder,
			ImagePullPolicy: corev1.PullAlways,
			Env:             envVars,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      git.BuildContextVolumeName,
					MountPath: git.Workspace,
				},
				{
					Name:      azureServicePrincipleSecretName,
					MountPath: azureServicePrincipleContext,
					ReadOnly:  true,
				},
			},
		}
		containers = append(containers, container)
	}

	pipelineInfo.ComponentImages = componentImages
	return containers
}

func getBuildContextComponents(components []v1.RadixComponent) map[string][]componentType {
	buildContextComponents := make(map[string][]componentType)

	for _, c := range components {
		if c.Image != "" {
			// Using public image. Nothing to build
			continue
		}

		componentSource := getDockerfile(c.SourceFolder, c.DockerfileName)
		components := buildContextComponents[componentSource]
		if components == nil {
			components = make([]componentType, 0)
		}

		components = append(components, componentType{c.Name, getContext(c.SourceFolder), getDockerfileName(c.DockerfileName)})
		buildContextComponents[componentSource] = components
	}

	return buildContextComponents
}
