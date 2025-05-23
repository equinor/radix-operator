package git

import (
	"fmt"
	"path"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	// The script to ensure that GitHub responds before cloning. It breaks after max attempts
	waitForGithubToRespond = "n=1;max=10;delay=2;while true; do if [ \"$n\" -lt \"$max\" ]; then nslookup github.com && break; n=$((n+1)); sleep $(($delay*$n)); else echo \"The command has failed after $n attempts.\"; break; fi done"
	podLabelsVolumeName    = "pod-labels"
	podLabelsFileName      = "labels"
)

// CloneConfig Git repository cloning configuration
type CloneConfig struct {
	NSlookupImage string
	GitImage      string
	BashImage     string
}

// CloneInitContainersWithSourceCode The sidecars for cloning repo with source code
func CloneInitContainersWithSourceCode(sshURL, branch string, config CloneConfig, workspace string) []corev1.Container {
	return CloneInitContainersWithContainerName(sshURL, branch, CloneContainerName, config, true, workspace)
}

// CloneInitContainersWithContainerName The sidecars for cloning repo. Lfs is to support large files in cloned source code, it is not needed for Radix config ot SubPipeline
func CloneInitContainersWithContainerName(sshURL, branch, cloneContainerName string, config CloneConfig, useLfs bool, workspace string) []corev1.Container {
	gitConfigCommand := fmt.Sprintf("git config --global --add safe.directory %s", workspace)
	gitCloneCommand := fmt.Sprintf("git clone %s -b %s --verbose --progress %s && (git submodule update --init --recursive || echo \"Warning: Unable to clone submodules, proceeding without them\")", sshURL, branch, workspace)
	getLfsFilesCommands := fmt.Sprintf("cd %s && if [ -n \"$(git lfs ls-files 2>/dev/null)\" ]; then git lfs install && echo 'Pulling large files...' && git lfs pull && echo 'Done'; fi && cd -", workspace)
	gitCloneCmd := []string{"sh", "-c", fmt.Sprintf("%s && %s %s", gitConfigCommand, gitCloneCommand,
		utils.TernaryString(useLfs, fmt.Sprintf("&& %s", getLfsFilesCommands), ""))}
	containers := []corev1.Container{
		{
			Name:    fmt.Sprintf("%snslookup", InternalContainerPrefix),
			Image:   config.NSlookupImage,
			Command: []string{"/bin/sh", "-c"},
			Args:    []string{waitForGithubToRespond},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewScaledQuantity(10, resource.Milli),
					corev1.ResourceMemory: *resource.NewScaledQuantity(10, resource.Mega),
				},
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    *resource.NewScaledQuantity(10, resource.Milli),
					corev1.ResourceMemory: *resource.NewScaledQuantity(50, resource.Mega),
				},
			},
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: securitycontext.Container(
				securitycontext.WithContainerRunAsUser(1000), // Any user will probably do
				securitycontext.WithContainerRunAsGroup(1000),
				securitycontext.WithContainerDropAllCapabilities(),
				securitycontext.WithContainerSeccompProfileType(corev1.SeccompProfileTypeRuntimeDefault),
				securitycontext.WithReadOnlyRootFileSystem(pointers.Ptr(true)),
			),
		},
		{
			Name:            cloneContainerName,
			Image:           config.GitImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         gitCloneCmd,
			Env: []corev1.EnvVar{
				{
					Name:  "HOME",
					Value: CloneRepoHomeVolumePath,
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      BuildContextVolumeName,
					MountPath: workspace,
				},
				{
					Name:      GitSSHKeyVolumeName,
					MountPath: "/.ssh",
					ReadOnly:  true,
				},
				{
					Name:      CloneRepoHomeVolumeName,
					MountPath: CloneRepoHomeVolumePath,
					ReadOnly:  false,
				},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewScaledQuantity(100, resource.Milli),
					corev1.ResourceMemory: *resource.NewScaledQuantity(250, resource.Mega),
				},
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    *resource.NewScaledQuantity(1000, resource.Milli),
					corev1.ResourceMemory: *resource.NewScaledQuantity(2000, resource.Mega),
				},
			},
			SecurityContext: securitycontext.Container(
				securitycontext.WithContainerRunAsUser(65534), // Must be this user when running as non root
				securitycontext.WithContainerRunAsGroup(1000),
				securitycontext.WithContainerDropAllCapabilities(),
				securitycontext.WithContainerSeccompProfileType(corev1.SeccompProfileTypeRuntimeDefault),
				securitycontext.WithReadOnlyRootFileSystem(pointers.Ptr(true)),
			),
		},
		{
			Name:            fmt.Sprintf("%schmod", InternalContainerPrefix),
			Image:           config.BashImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         []string{"/usr/local/bin/bash", "-O", "dotglob", "-c"},
			Args:            []string{fmt.Sprintf("chmod -R g+rw %s", path.Join(workspace, "*"))},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      BuildContextVolumeName,
					MountPath: workspace,
				},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewScaledQuantity(10, resource.Milli),
					corev1.ResourceMemory: *resource.NewScaledQuantity(1, resource.Mega),
				},
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    *resource.NewScaledQuantity(50, resource.Milli),
					corev1.ResourceMemory: *resource.NewScaledQuantity(100, resource.Mega),
				},
			},
			SecurityContext: securitycontext.Container(
				securitycontext.WithContainerRunAsUser(65534), // Must be this user when running as non root
				securitycontext.WithContainerRunAsGroup(1000),
				securitycontext.WithContainerDropAllCapabilities(),
				securitycontext.WithContainerSeccompProfileType(corev1.SeccompProfileTypeRuntimeDefault),
				securitycontext.WithReadOnlyRootFileSystem(pointers.Ptr(true)),
			),
		},
	}

	return containers
}

// GetJobVolumes Get Radix pipeline job volumes
func GetJobVolumes() []corev1.Volume {
	defaultMode := int32(256)

	volumes := []corev1.Volume{
		{
			Name: BuildContextVolumeName,
		},
		{
			Name: CloneRepoHomeVolumeName,
		},
		{
			Name: GitSSHKeyVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  GitSSHKeyVolumeName,
					DefaultMode: &defaultMode,
				},
			},
		},
		{
			Name: podLabelsVolumeName,
			VolumeSource: corev1.VolumeSource{
				DownwardAPI: &corev1.DownwardAPIVolumeSource{
					Items: []corev1.DownwardAPIVolumeFile{
						{
							Path:     podLabelsFileName,
							FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.labels"},
						},
					},
				},
			},
		},
	}
	return volumes
}

// GetJobContainerVolumeMounts Get job container volume mounts
func GetJobContainerVolumeMounts(workspace string) []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      BuildContextVolumeName,
			MountPath: workspace,
		},
		{
			Name:      podLabelsVolumeName,
			MountPath: fmt.Sprintf("/%s", podLabelsVolumeName),
		},
	}
}
