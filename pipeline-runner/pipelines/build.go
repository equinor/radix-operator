package onpush

import (
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (cli *RadixOnPushHandler) build(radixRegistration *v1.RadixRegistration, radixApplication *v1.RadixApplication, branch, imageTag string) error {
	appName := radixRegistration.Name
	cloneURL := radixRegistration.Spec.CloneURL
	namespace := fmt.Sprintf("%s-app", appName)
	log.Infof("building app %s", appName)
	// TODO - what about build secrets, e.g. credentials for private npm repository?
	job, err := createBuildJob(appName, radixApplication.Spec.Components, cloneURL, branch, imageTag)
	if err != nil {
		return err
	}

	// should we only build if the git commit hash differ from what exist earlier?
	// include git commit hash as label on job (registered in app namespace), check if it exist and is completeded earlier
	// do we need to also query ACR and see if image exist?
	log.Infof("Apply job (%s) to build components for app %s", job.Name, appName)
	job, err = cli.kubeclient.BatchV1().Jobs(namespace).Create(job)
	if err != nil {
		return err
	}

	watcher, err := cli.kubeclient.BatchV1().Jobs(namespace).Watch(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("build=%s", fmt.Sprintf("%s-%s", appName, imageTag)),
	})
	if err != nil {
		return err
	}
	channel := watcher.ResultChan()
	done := make(chan error)

	go func() {
		for event := range channel {
			if event.Type == "ADDED" || event.Type == "MODIFIED" {
				jobModified, _ := event.Object.(*batchv1.Job)
				if jobModified.Status.Succeeded == 1 {
					err = nil
					done <- nil
					return
				}
				if jobModified.Status.Failed == 1 {
					err = fmt.Errorf("Build job failed")
					done <- err
					return
				}
			} else if event.Type == "ERROR" {
				err = fmt.Errorf("Error watching job: %v", event.Object)
				done <- err
				return
			} else if event.Type == "DELETED" {
				err = fmt.Errorf("Error, job deleted: %v", event.Object)
				done <- err
				return
			}
		}
	}()

	<-done
	return err
}

const workspace = "/workspace"

func createBuildJob(appName string, components []v1.RadixComponent, cloneURL, branch, imageTag string) (*batchv1.Job, error) {
	gitCloneCommand := fmt.Sprintf("git clone %s -b %s .", cloneURL, branch) // TODO - MUST ensure commands are not injected
	buildContainers := createBuildContainers(appName, imageTag, components)

	defaultMode, backOffLimit := int32(256), int32(1)

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("radix-builder-%s", imageTag),
			Labels: map[string]string{
				"build": fmt.Sprintf("%s-%s", appName, imageTag),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backOffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: "Never",
					InitContainers: []corev1.Container{
						{
							Name:    "clone",
							Image:   "alpine:3.7",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{fmt.Sprintf("apk add --no-cache bash openssh-client git && ls /root/.ssh && cd %s && %s", workspace, gitCloneCommand)},
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

func createBuildContainers(appName, imageTag string, components []v1.RadixComponent) []corev1.Container {
	containers := []corev1.Container{}

	for _, c := range components {
		imagePath := getImagePath(appName, c.Name, imageTag)
		dockerFile := getDockerfile(c.SourceFolder, c.DockerfileName)
		context := getContext(c.SourceFolder)
		log.Infof("using dockerfile %s in context %s", dockerFile, context)
		container := corev1.Container{
			Name:  fmt.Sprintf("build-%s", c.Name),
			Image: "gcr.io/kaniko-project/executor:latest", // todo - version?
			Args: []string{
				fmt.Sprintf("--dockerfile=%s", dockerFile),
				fmt.Sprintf("--context=%s", context),
				fmt.Sprintf("--destination=%s", imagePath),
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
