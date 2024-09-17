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

	// BuildHomeVolumeName Name of volume to hold build home folder
	BuildHomeVolumeName = "builder-home"

	// BuildHomeVolumePath Name of home volume path
	BuildHomeVolumePath = "/home/builder"

	// Workspace Folder to hold the code to build
	Workspace = "/workspace"
)
