package git

import (
	"fmt"
	"path"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	// InternalContainerPrefix To indicate that this is not for user interest
	InternalContainerPrefix = "internal-"

	// CloneConfigContainerName Name of container for clone in the outer pipeline
	CloneConfigContainerName = "clone-config"

	// CloneContainerName Name of container
	CloneContainerName = "clone"

	// GitSSHKeyVolumeName Deploy key + known_hosts
	GitSSHKeyVolumeName = "git-ssh-keys"

	// BuildContextVolumeName Name of volume to hold build context
	BuildContextVolumeName = "build-context"

	// Workspace Folder to hold the code to build
	Workspace = "/workspace"

	// The script to ensure that github responds before cloning. It breaks after max attempts
	waitForGithubToRespond = "n=1;max=10;delay=2;while true; do if [ \"$n\" -lt \"$max\" ]; then nslookup github.com && break; n=$((n+1)); sleep $(($delay*$n)); else echo \"The command has failed after $n attempts.\"; break; fi done"
)

// CloneInitContainers The sidecars for cloning repo
func CloneInitContainers(sshURL, branch string) []corev1.Container {
	return CloneInitContainersWithContainerName(sshURL, branch, CloneContainerName)
}

// CloneInitContainersWithContainerName The sidecars for cloning repo
func CloneInitContainersWithContainerName(sshURL, branch, cloneContainerName string) []corev1.Container {
	gitCloneCmd := []string{"git", "clone", "--recurse-submodules", sshURL, "-b", branch, "--verbose", "--progress", Workspace}
	containers := []corev1.Container{
		{
			Name:            fmt.Sprintf("%snslookup", InternalContainerPrefix),
			Image:           "alpine",
			ImagePullPolicy: corev1.PullAlways,
			Args:            []string{waitForGithubToRespond},
			Command:         []string{"/bin/sh", "-c"},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewScaledQuantity(10, resource.Milli),
					corev1.ResourceMemory: *resource.NewScaledQuantity(1, resource.Mega),
				},
			},
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
			Image:           "alpine/git:2.45.1",
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         gitCloneCmd,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      BuildContextVolumeName,
					MountPath: Workspace,
				},
				{
					Name:      GitSSHKeyVolumeName,
					MountPath: "/.ssh",
					ReadOnly:  true,
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
			Image:           "bash",
			ImagePullPolicy: corev1.PullAlways,
			Command:         []string{"/usr/local/bin/bash", "-O", "dotglob", "-c"},
			Args:            []string{fmt.Sprintf("chmod -R g+rw %s", path.Join(Workspace, "*"))},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      BuildContextVolumeName,
					MountPath: Workspace,
				},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewScaledQuantity(10, resource.Milli),
					corev1.ResourceMemory: *resource.NewScaledQuantity(1, resource.Mega),
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
