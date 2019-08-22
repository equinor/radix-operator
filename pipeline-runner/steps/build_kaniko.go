package steps

import (
	"fmt"
	"strings"
	"time"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// builds using kaniko - not currently used because of slowness and bugs
func createKanikoBuildJob(rr *v1.RadixRegistration, ra *v1.RadixApplication, containerRegistry string, pipelineInfo *model.PipelineInfo) (*batchv1.Job, error) {
	appName := rr.Name
	branch := pipelineInfo.PipelineArguments.Branch
	imageTag := pipelineInfo.PipelineArguments.ImageTag
	jobName := pipelineInfo.PipelineArguments.JobName

	initContainers := git.CloneInitContainers(rr.Spec.CloneURL, branch)
	buildContainers := createBuildContainers(containerRegistry, appName, imageTag, pipelineInfo.PipelineArguments.UseCache, ra.Spec.Components)
	timestamp := time.Now().Format("20060102150405")

	defaultMode, backOffLimit := int32(256), int32(0)

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
				kube.RadixBranchAnnotation: branch,
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
						{
							Name: git.GitSSHKeyVolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  git.GitSSHKeyVolumeName,
									DefaultMode: &defaultMode,
								},
							},
						},
						{
							Name: "docker-config",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "radix-docker",
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

func createBuildContainers(containerRegistry, appName, imageTag, useCache string, components []v1.RadixComponent) []corev1.Container {
	containers := []corev1.Container{}

	for _, c := range components {
		if c.Image != "" {
			// Using public image. Nothing to build
			continue
		}

		imagePath := utils.GetImagePath(containerRegistry, appName, c.Name, imageTag)
		dockerFile := getDockerfile(c.SourceFolder, c.DockerfileName)
		context := getContext(c.SourceFolder)
		log.Infof("using dockerfile %s in context %s", dockerFile, context)
		container := corev1.Container{
			Name:  fmt.Sprintf("build-%s", c.Name),
			Image: "gcr.io/kaniko-project/executor:v0.10.0", // todo - version?
			Args: []string{
				fmt.Sprintf("--dockerfile=%s", dockerFile),
				fmt.Sprintf("--context=%s", context),
				fmt.Sprintf("--destination=%s", imagePath),
				fmt.Sprintf("--cache=%s", useCache),
				"--snapshotMode=time",
			},
			Env: []corev1.EnvVar{
				{
					Name:  "DOCKER_CONFIG",
					Value: "/kaniko/secrets",
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      git.BuildContextVolumeName,
					MountPath: git.Workspace,
				},
				{
					Name:      "docker-config",
					MountPath: "/kaniko/secrets",
					ReadOnly:  true,
				},
			},
		}
		containers = append(containers, container)
	}
	return containers
}

func getDockerfile(sourceFolder, dockerfileName string) string {
	context := getContext(sourceFolder)
	if dockerfileName == "" {
		dockerfileName = "Dockerfile"
	}

	return fmt.Sprintf("%s%s", context, dockerfileName)
}

func getContext(sourceFolder string) string {
	sourceFolder = strings.Trim(sourceFolder, ".")
	sourceFolder = strings.Trim(sourceFolder, "/")
	if sourceFolder == "" {
		return fmt.Sprintf("%s/", git.Workspace)
	}
	return fmt.Sprintf("%s/%s/", git.Workspace, sourceFolder)
}

func getGitCloneCommand(cloneURL, branch string) string {
	return fmt.Sprintf("git clone %s -b %s .", cloneURL, branch)
}

func getInitContainerArgString(workspace, gitCloneCommand, commitID string) string {
	argString := fmt.Sprintf("apk add --no-cache bash openssh-client git && ls /root/.ssh && cd %s && %s", workspace, gitCloneCommand)
	if commitID != "" {
		checkoutString := fmt.Sprintf("git checkout %s", commitID)
		argString += " && " + checkoutString
	}
	return argString
}
