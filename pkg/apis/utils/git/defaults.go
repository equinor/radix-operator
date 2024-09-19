package git

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

	// CloneRepoHomeVolumeName Name of volume to hold clone repo home folder
	CloneRepoHomeVolumeName = "builder-home"

	// CloneRepoHomeVolumePath Name of home volume path
	CloneRepoHomeVolumePath = "/home/clone"

	// Workspace Folder to hold the code to build
	Workspace = "/workspace"
)
