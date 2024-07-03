package git

import (
	"github.com/equinor/radix-operator/pipeline-runner/model"
	gitclone "github.com/equinor/radix-operator/pkg/apis/utils/git"
)

func CloneConfigFromPipelineArgs(args model.PipelineArguments) gitclone.CloneConfig {
	return gitclone.CloneConfig{
		NSlookupImage: args.GitCloneNsLookupImage,
		GitImage:      args.GitCloneGitImage,
		BashImage:     args.GitCloneBashImage,
	}
}
