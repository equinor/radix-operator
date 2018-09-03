package onpush

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (cli *RadixOnPushHandler) build(radixRegistration *v1.RadixRegistration, radixApplication *v1.RadixApplication, branch, imageTag string) error {
	appName := radixRegistration.Name
	namespace := fmt.Sprintf("%s-app", appName)
	job, err := createBuildJob(appName, radixApplication.Spec.Components, radixRegistration.Spec.CloneURL, branch, imageTag)
	if err != nil {
		return err
	}

	// should we only build if the git commit hash differ from what exist earlier?
	// include git commit hash as label on job (registered in app namespace), check if it exist and is completeded earlier
	// do we need to also query ACR and see if image exist?
	job, err = cli.kubeclient.BatchV1().Jobs(namespace).Create(job)
	if err != nil {
		return err
	}

	watcher, err := cli.kubeclient.BatchV1().Jobs(namespace).Watch(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", appName),
	})
	if err != nil {
		return err
	}
	channel := watcher.ResultChan()

	for event := range channel {
		if event.Type == "ADDED" || event.Type == "MODIFIED" {
			jobModified, _ := event.Object.(*batchv1.Job)
			if jobModified.Status.Succeeded == 1 {
				return nil
			}
			if jobModified.Status.Failed == 1 {
				return fmt.Errorf("Build job failed")
			}
		}
	}
	return nil
}

func createBuildJob(appName string, components []v1.RadixComponent, cloneURL, branch, imageTag string) (*batchv1.Job, error) {
	gitCloneCommand := fmt.Sprintf("git clone %s -b %s .", cloneURL, branch) // TODO - MUST ensure commands are not injected
	buildContainers := createBuildContainers(appName, imageTag, components)

	defaultMode, backOffLimit := int32(256), int32(1)

	log.Infof("cloning from (%s): %s", branch, cloneURL)
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", appName, imageTag), // todo - job name - bind it to version?
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
									SecretName: "docker-config",
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
		dockerFile := fmt.Sprintf("/workspace/%s", c.SourceFolder)
		container := corev1.Container{
			Name:  fmt.Sprintf("build-%s", c.Name),
			Image: "gcr.io/kaniko-project/executor:latest", // todo - version?
			Args: []string{
				fmt.Sprintf("--dockerfile=%s/Dockerfile", dockerFile),
				fmt.Sprintf("--context=%s", dockerFile),
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
					MountPath: "/workspace",
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
