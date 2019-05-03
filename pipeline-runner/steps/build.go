package steps

import (
	"fmt"
	"strings"
	"time"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type BuildHandler struct {
	kubeclient  kubernetes.Interface
	radixclient radixclient.Interface
	kubeutil    *kube.Kube
}

// Init constructor
func InitBuildHandler(kubeclient kubernetes.Interface, radixclient radixclient.Interface) BuildHandler {
	kube, _ := kube.New(kubeclient)
	return BuildHandler{
		kubeclient:  kubeclient,
		radixclient: radixclient,
		kubeutil:    kube,
	}
}

func (cli BuildHandler) SucceededMsg(pipelineInfo model.PipelineInfo) string {
	return fmt.Sprintf("Succeded: build docker image for application %s", pipelineInfo.GetAppName())
}

func (cli BuildHandler) ErrorMsg(pipelineInfo model.PipelineInfo, err error) string {
	return fmt.Sprintf("Failed to build application %s. Error: %v", pipelineInfo.GetAppName(), err)
}

func (cli BuildHandler) Run(pipelineInfo model.PipelineInfo) error {
	appName := pipelineInfo.GetAppName()
	namespace := utils.GetAppNamespace(appName)
	containerRegistry, err := cli.kubeutil.GetContainerRegistry()
	if err != nil {
		return err
	}

	log.Infof("Building app %s", appName)
	// TODO - what about build secrets, e.g. credentials for private npm repository?
	job, err := createACRBuildJob(containerRegistry, pipelineInfo)
	if err != nil {
		return err
	}

	log.Infof("Apply job (%s) to build components for app %s", job.Name, appName)
	job, err = cli.kubeclient.BatchV1().Jobs(namespace).Create(job)
	if err != nil {
		return err
	}

	return cli.watchJob(job)
}

func (cli BuildHandler) watchJob(job *batchv1.Job) error {
	errChan := make(chan error)
	stop := make(chan struct{})
	defer close(stop)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		cli.kubeclient, 0, kubeinformers.WithNamespace(job.GetNamespace()))
	informer := kubeInformerFactory.Batch().V1().Jobs().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new interface{}) {
			j, success := new.(*batchv1.Job)
			if success && job.GetName() == j.GetName() && job.GetNamespace() == j.GetNamespace() {
				switch {
				case j.Status.Succeeded == 1:
					errChan <- nil
				case j.Status.Failed == 1:
					errChan <- fmt.Errorf("Build docker image failed. See build log")
				default:
					log.Debugf("Ongoing - build docker image")
				}
			}
		},
		DeleteFunc: func(old interface{}) {
			j, success := old.(*batchv1.Job)
			if success && j.GetName() == job.GetName() && job.GetNamespace() == j.GetNamespace() {
				errChan <- fmt.Errorf("Build failed - Job deleted!")
			}
		},
	})

	go informer.Run(stop)
	if !cache.WaitForCacheSync(stop, informer.HasSynced) {
		errChan <- fmt.Errorf("Timed out waiting for caches to sync")
	}

	err := <-errChan
	return err
}

const workspace = "/workspace"

// builds using kaniko - not currently used because of slowness and bugs
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
