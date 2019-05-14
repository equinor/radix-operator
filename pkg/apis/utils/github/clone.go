package github

import corev1 "k8s.io/api/core/v1"

// CloneInitContainers The sidecars for cloning repo
func CloneInitContainers(sshURL, branch, buildContextVolumeName, workspace, gitSSHKeyVolumeName string) []corev1.Container {
	containers := []corev1.Container{
		{
			Name:  "nslookup",
			Image: "alpine",
			Args: []string{
				"n=1;max=10;delay=5;while true; do if [ '$n' -lt '$max' ]; then nslookup github.com && break; n=$((n+1)); sleep $(($delay*$n)); else echo 'The command has failed after $n attempts.'; break; fi done"},
			ImagePullPolicy: "Always",
		},
		{
			Name:  "clone",
			Image: "alpine/git",
			Args: []string{
				"clone",
				sshURL,
				"-b",
				branch,
				"--verbose",
				"--progress",
				"/workspace"},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      buildContextVolumeName,
					MountPath: workspace,
				},
				{
					Name:      gitSSHKeyVolumeName,
					MountPath: "/root/.ssh",
					ReadOnly:  true,
				},
			},
		},
	}

	return containers
}
