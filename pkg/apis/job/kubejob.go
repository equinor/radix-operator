package job

import (
	"fmt"
	"strings"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	pipelineJob "github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	workerImage     = "radix-pipeline"
	dockerRegistry  = "radixdev.azurecr.io"
	radixJobTypeJob = "job"
)

func (job *Job) createJob() error {
	namespace := job.radixJob.Namespace
	name := job.radixJob.Name

	ownerReference := getOwnerReference(job.radixJob)
	jobConfig, err := job.getJobConfig(name)
	if err != nil {
		return err
	}

	jobConfig.OwnerReferences = ownerReference
	_, err = job.kubeclient.BatchV1().Jobs(namespace).Create(jobConfig)
	if err != nil {
		return err
	}

	return nil
}

func (job *Job) getJobConfig(name string) (*batchv1.Job, error) {
	imageTag := fmt.Sprintf("%s/%s:%s", dockerRegistry, workerImage, job.radixJob.Spec.PipelineImage)

	log.Infof("Using image: %s", imageTag)

	backOffLimit := int32(0)

	appName := job.radixJob.Spec.AppName
	jobName := job.radixJob.Name
	randomStr := job.radixJob.Spec.ImageTag
	branch := job.radixJob.Spec.Branch
	commitID := job.radixJob.Spec.CommitID
	pushImage := job.radixJob.Spec.PushImage

	pipeline, err := pipelineJob.GetPipelineFromName(string(job.radixJob.Spec.PipeLineType))
	if err != nil {
		return nil, err
	}

	jobCfg := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   jobName,
			Labels: getPipelineJobLabels(appName, jobName, randomStr, branch, commitID, pipeline),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backOffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "radix-pipeline",
					InitContainers:     getPipelineJobInitContainers(job.registration.Spec.CloneURL, pipeline),
					Containers: []corev1.Container{
						{
							Name:            workerImage,
							Image:           imageTag,
							ImagePullPolicy: corev1.PullAlways,
							Args:            getPipelineJobArguments(appName, jobName, randomStr, branch, commitID, pushImage, pipeline),
							VolumeMounts:    getPipelineJobContainerVolumeMounts(pipeline),
						},
					},
					Volumes:       getPipelineJobVolumes(pipeline),
					RestartPolicy: "Never",
				},
			},
		},
	}

	return &jobCfg, nil
}

func getUniqueJobName(image string) (string, string) {
	var jobName []string
	randomStr := strings.ToLower(utils.RandString(5))
	jobName = append(jobName, image)
	jobName = append(jobName, "-")
	jobName = append(jobName, getCurrentTimestamp())
	jobName = append(jobName, "-")
	jobName = append(jobName, randomStr)
	return strings.Join(jobName, ""), randomStr
}

func getCurrentTimestamp() string {
	t := time.Now()
	return t.Format("20060102150405") // YYYYMMDDHHMISS in Go
}

func getPipelineJobInitContainers(sshURL string, pipeline *pipelineJob.Definition) []corev1.Container {
	var initContainers []corev1.Container

	switch pipeline.Name {
	case pipelineJob.BuildDeploy, pipelineJob.Build:
		initContainers = git.CloneInitContainers(sshURL, "master")
	}
	return initContainers
}

func getPipelineJobArguments(appName, jobName, randomStr, branch, commitID string, pushImage bool, pipeline *pipelineJob.Definition) []string {
	// Base arguments for all types of pipeline
	args := []string{
		fmt.Sprintf("JOB_NAME=%s", jobName),
		fmt.Sprintf("PIPELINE_TYPE=%s", pipeline.Name),
	}

	switch pipeline.Name {
	case pipelineJob.BuildDeploy:
		args = append(args, fmt.Sprintf("IMAGE_TAG=%s", randomStr))
		fallthrough
	case pipelineJob.Build:
		args = append(args, fmt.Sprintf("BRANCH=%s", branch))
		args = append(args, fmt.Sprintf("COMMIT_ID=%s", commitID))
		args = append(args, fmt.Sprintf("PUSH_IMAGE=%s", getPushImageTag(pushImage)))
		args = append(args, fmt.Sprintf("RADIX_FILE_NAME=%s", "/workspace/radixconfig.yaml"))
	case pipelineJob.Promote:
		args = append(args, fmt.Sprintf("RADIX_APP=%s", appName))
		args = append(args, fmt.Sprintf("DEPLOYMENT_NAME=%s", "TODO"))
		args = append(args, fmt.Sprintf("FROM_ENVIRONMENT=%s", "TODO"))
		args = append(args, fmt.Sprintf("TO_ENVIRONMENT=%s", "TODO"))
	}

	return args
}

func getPipelineJobLabels(appName, jobName, randomStr, branch, commitID string, pipeline *pipelineJob.Definition) map[string]string {
	// Base labels for all types of pipeline
	labels := map[string]string{
		kube.RadixJobNameLabel: jobName,
		kube.RadixJobTypeLabel: radixJobTypeJob,
		"radix-pipeline":       pipeline.Name,
		"radix-app-name":       appName, // For backwards compatibility. Remove when cluster is migrated
		kube.RadixAppLabel:     appName,
	}

	switch pipeline.Name {
	case pipelineJob.BuildDeploy:
		labels[kube.RadixImageTagLabel] = randomStr
		fallthrough
	case pipelineJob.Build:
		labels[kube.RadixBranchLabel] = branch
		labels[kube.RadixCommitLabel] = commitID
	}

	return labels
}

func getPipelineJobContainerVolumeMounts(pipeline *pipelineJob.Definition) []corev1.VolumeMount {
	var volumeMounts []corev1.VolumeMount

	switch pipeline.Name {
	case pipelineJob.BuildDeploy, pipelineJob.Build:
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      git.BuildContextVolumeName,
			MountPath: git.Workspace,
		})
	}

	return volumeMounts
}

func getPipelineJobVolumes(pipeline *pipelineJob.Definition) []corev1.Volume {
	var volumes []corev1.Volume
	defaultMode := int32(256)

	switch pipeline.Name {
	case pipelineJob.BuildDeploy, pipelineJob.Build:
		volumes = append(volumes, corev1.Volume{
			Name: git.BuildContextVolumeName,
		})
		volumes = append(volumes, corev1.Volume{
			Name: git.GitSSHKeyVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  git.GitSSHKeyVolumeName,
					DefaultMode: &defaultMode,
				},
			},
		})
	}

	return volumes
}

func getPushImageTag(pushImage bool) string {
	if pushImage {
		return "1"
	}

	return "0"
}
