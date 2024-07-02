package git_test

import (
	"testing"

	internalgit "github.com/equinor/radix-operator/pipeline-runner/internal/git"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	"github.com/stretchr/testify/assert"
)

func Test_CloneConfigFromPipelineArgs(t *testing.T) {
	args := model.PipelineArguments{GitCloneNsLookupImage: "anynslookup:any", GitCloneGitImage: "anygit:any", GitCloneBashImage: "anybash:any"}
	actual := internalgit.CloneConfigFromPipelineArgs(args)
	expected := git.CloneConfig{NSlookupImage: args.GitCloneNsLookupImage, GitImage: args.GitCloneGitImage, BashImage: args.GitCloneBashImage}
	assert.Equal(t, expected, actual)
}
