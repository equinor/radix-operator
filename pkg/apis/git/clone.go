package git

import (
	"fmt"
	"path"
	"strings"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
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
// Arguments:
//   - sshURL: SSH URL to the remote git repository
//   - branch: The branch to checkout
//   - commitID (optional): The commit ID to checkout after clone finished. The clone step will fail if the commit ID is not an ancestor of HEAD for the branch
//   - directory: The directory to clone the git repository to
//   - config: defines the container images to be used by the init containers
func CloneInitContainersWithSourceCode(sshURL, branch, commitID, directory string, config CloneConfig) []corev1.Container {
	return CloneInitContainersWithContainerName(sshURL, branch, commitID, directory, true, true, CloneContainerName, config)
}

// CloneInitContainersWithContainerName The sidecars for cloning a git repo. Lfs is to support large files in cloned source code, it is not needed for Radix config ot SubPipeline
func CloneInitContainersWithContainerName(sshURL, branch, commitID, directory string, useLfs, skipBlobs bool, cloneContainerName string, config CloneConfig) []corev1.Container {
	commands := []string{
		fmt.Sprintf("git config --global --add safe.directory %s", directory),
	}

	cloneArgs := []string{
		fmt.Sprintf("-b %s", branch),
		"--verbose",
		"--progress",
	}
	if skipBlobs {
		cloneArgs = append(cloneArgs, "--filter=blob:none")
	}
	commands = append(commands, fmt.Sprintf("git clone %s %s %s && (git submodule update --init --recursive || echo \"Warning: Unable to clone submodules, proceeding without them\")", sshURL, strings.Join(cloneArgs, " "), directory))

	if len(commitID) > 0 {
		commands = append(commands, fmt.Sprintf("cd %[1]s && echo \"Checking out commit %[2]s\" && git merge-base --is-ancestor %[2]s HEAD && git checkout -q %[2]s && cd -", directory, commitID))
	}

	if useLfs {
		commands = append(commands, fmt.Sprintf("cd %s && if [ -n \"$(git lfs ls-files 2>/dev/null)\" ]; then git lfs install && echo 'Pulling large files...' && git lfs pull && echo 'Done'; fi && cd -", directory))
	}

	gitCloneCmd := []string{"sh", "-c", strings.Join(commands, " && ")}

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
					MountPath: directory,
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
			Args:            []string{fmt.Sprintf("chmod -R g+rw %s", path.Join(directory, "*"))},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      BuildContextVolumeName,
					MountPath: directory,
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
