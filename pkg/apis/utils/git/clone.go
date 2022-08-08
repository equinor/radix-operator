package git

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
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

	// The script to ensure that github reponds before cloning. It breaks after max attempts
	waitForGithubToRespond = "n=1;max=10;delay=2;while true; do if [ \"$n\" -lt \"$max\" ]; then nslookup github.com && break; n=$((n+1)); sleep $(($delay*$n)); else echo \"The command has failed after $n attempts.\"; break; fi done"
)

func getGitCloneCommand(sshURL, branch, webhookCommitId string) string {
	gitClone := fmt.Sprintf("git clone --recurse-submodules %s --single-branch -b %s --verbose --progress %s", sshURL, branch, Workspace)
	return gitClone
	// TODO: Decide how/if we shall handle commitId for radixconfig
	// if webhookCommitId == "" {
	// 	return gitClone
	// }
	// log.Debugf("webhookCommitId was non-empty, checking out commit %s in initContainer for prepare-pipelines step", webhookCommitId)
	// gitCloneAndCheckout := fmt.Sprintf("%s && git --git-dir %s/.git --work-tree %s checkout %s", gitClone, Workspace, Workspace, webhookCommitId)
	// return gitCloneAndCheckout
}

// CloneInitContainers The sidecars for cloning repo
func CloneInitContainers(sshURL, branch string, containerSecContext corev1.SecurityContext, webhookCommitId string) []corev1.Container {
	return CloneInitContainersWithContainerName(sshURL, branch, CloneContainerName, containerSecContext, webhookCommitId)
}

// CloneInitContainersWithContainerName The sidecars for cloning repo
func CloneInitContainersWithContainerName(sshURL, branch, cloneContainerName string, containerSecContext corev1.SecurityContext, webhookCommitId string) []corev1.Container {
	gitCloneCmd := getGitCloneCommand(sshURL, branch, webhookCommitId)
	containers := []corev1.Container{
		{
			Name:            fmt.Sprintf("%snslookup", InternalContainerPrefix),
			Image:           "alpine",
			Args:            []string{waitForGithubToRespond},
			Command:         []string{"/bin/sh", "-c"},
			ImagePullPolicy: "Always",
			SecurityContext: &containerSecContext,
		},
		{
			Name:            cloneContainerName,
			Image:           "alpine/git:user",
			ImagePullPolicy: "IfNotPresent",
			Command:         []string{"/bin/sh", "-c"},
			Args:            []string{gitCloneCmd},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      BuildContextVolumeName,
					MountPath: Workspace,
				},
				{
					Name:      GitSSHKeyVolumeName,
					MountPath: "/home/git-user/.ssh",
					ReadOnly:  true,
				},
			},
			SecurityContext: &containerSecContext,
		},
	}

	return containers
}
