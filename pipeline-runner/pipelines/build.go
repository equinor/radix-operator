package onpush

import (
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (cli *RadixOnPushHandler) build(jobName string, radixRegistration *v1.RadixRegistration, radixApplication *v1.RadixApplication, branch, commitID, imageTag, useCache string) error {
	appName := radixRegistration.Name
	cloneURL := radixRegistration.Spec.CloneURL
	namespace := utils.GetAppNamespace(appName)
	containerRegistry, err := cli.kubeutil.GetContainerRegistry()
	if err != nil {
		return err
	}

	log.Infof("building app %s", appName)
	// TODO - what about build secrets, e.g. credentials for private npm repository?
	job, err := createACRBuildJob(containerRegistry, appName, jobName, radixApplication.Spec.Components, cloneURL, branch, commitID, imageTag, useCache)
	if err != nil {
		return err
	}

	log.Infof("Apply job (%s) to build components for app %s", job.Name, appName)
	job, err = cli.kubeclient.BatchV1().Jobs(namespace).Create(job)
	if err != nil {
		return err
	}

	// TODO: workaround watcher bug - pull job status instead of watching
	done := make(chan error)
	go func() {
		for {
			j, err := cli.kubeclient.BatchV1().Jobs(namespace).Get(job.Name, metav1.GetOptions{})
			if err != nil {
				done <- err
				return
			}
			switch {
			case j.Status.Succeeded == 1:
				done <- nil
				return
			case j.Status.Failed == 1:
				done <- fmt.Errorf("Build docker image failed")
				return
			default:
				log.Debugf("Ongoing - build docker image")
			}
			time.Sleep(5 * time.Second)
		}
	}()
	err = <-done

	return err
}

const workspace = "/workspace"

func createBuildJob(containerRegistry, appName, jobName string, components []v1.RadixComponent, cloneURL, branch, commitID, imageTag, useCache string) (*batchv1.Job, error) {
	gitCloneCommand := getGitCloneCommand(cloneURL, branch)
	argString := getInitContainerArgString(workspace, gitCloneCommand, commitID)
	buildContainers := createBuildContainers(containerRegistry, appName, imageTag, useCache, components)
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
				kube.RadixBranchLabel:   branch,
				kube.RadixJobTypeLabel:  kube.RadixJobTypeBuild,
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
							Name:    "clone",
							Image:   "alpine:3.7",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{argString},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "build-context",
									MountPath: workspace,
								},
								{
									Name:      "git-ssh-keys",
									MountPath: "/root/.ssh",
									ReadOnly:  true,
								},
							},
						},
					},
					Containers: buildContainers,
					Volumes: []corev1.Volume{
						{
							Name: "build-context",
						},
						{
							Name: "git-ssh-keys",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  "git-ssh-keys",
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
		imagePath := utils.GetImagePath(containerRegistry, appName, c.Name, imageTag)
		dockerFile := getDockerfile(c.SourceFolder, c.DockerfileName)
		context := getContext(c.SourceFolder)
		log.Infof("using dockerfile %s in context %s", dockerFile, context)
		container := corev1.Container{
			Name:  fmt.Sprintf("build-%s", c.Name),
			Image: "gcr.io/kaniko-project/executor:v0.7.0", // todo - version?
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
					Name:      "build-context",
					MountPath: workspace,
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
		return fmt.Sprintf("%s/", workspace)
	}
	return fmt.Sprintf("%s/%s/", workspace, sourceFolder)
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
