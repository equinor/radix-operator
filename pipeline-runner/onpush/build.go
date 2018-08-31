package onpush

import (
	"errors"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (cli *RadixOnPushHandler) build(radixRegistration *v1.RadixRegistration, branch string) (string, error) {
	appName := radixRegistration.Name
	namespace := fmt.Sprintf("%s-app", appName)
	job, imagePath, err := createBuildJob(appName, radixRegistration.Spec.CloneURL, branch)
	if err != nil {
		return "", err
	}

	// should we only build if the git commit hash differ from what exist earlier?
	// include git commit hash as label on job (registered in app namespace), check if it exist and is completeded earlier
	// do we need to also query ACR and see if image exist?
	job, err = cli.kubeclient.BatchV1().Jobs(namespace).Create(job)
	if err != nil {
		return "", err
	}

	// wait for it to finish
	watcher, _ := cli.kubeclient.BatchV1().Jobs(namespace).Watch(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%", appName),
	})
	channel := watcher.ResultChan()

	for event := range channel {
		if event.Type == "ADDED" || event.Type == "MODIFIED" {
			jobModified, _ := event.Object.(*batchv1.Job)
			if jobModified.Status.Succeeded == 1 {
				return imagePath, nil
			}
			if jobModified.Status.Failed == 1 {
				return "", errors.New("Build job failed!")
			}
		}
		log.Info(event)
	}
	// return imagePath
	return imagePath, nil
}

func createBuildJob(appName, cloneURL, branch string) (*batchv1.Job, string, error) {
	containerRegistryURL := "radixdev.azurecr.io" // const
	imagePath := fmt.Sprintf("%s/%s:kaniko", containerRegistryURL, appName)
	gitCloneCommand := fmt.Sprintf("git clone %s -b %s .", cloneURL, branch) // TODO - MUST ensure commands are not injected
	defaultMode := int32(256)
	backOffLimit := int32(1)
	log.Infof("cloning from (%s): %s", branch, cloneURL)

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: appName,
			Labels: map[string]string{
				"app": appName,
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
							Args:    []string{fmt.Sprintf("apk add --no-cache bash openssh-client git && ls /root/.ssh && cd /workspace && %s", gitCloneCommand)},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "build-context",
									MountPath: "/workspace",
								},
								{
									Name:      "git-ssh-keys",
									MountPath: "/root/.ssh",
									ReadOnly:  true,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "build",
							Image: "gcr.io/kaniko-project/executor:latest",
							Args: []string{
								"--context=/workspace/",
								"--destination=" + imagePath,
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
									MountPath: "/workspace",
								},
								{
									Name:      "docker-config",
									MountPath: "/kaniko/secrets",
									ReadOnly:  true,
								},
							},
						},
					},
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
									SecretName: "docker-config",
								},
							},
						},
					},
				},
			},
		},
	}
	return &job, imagePath, nil
}
