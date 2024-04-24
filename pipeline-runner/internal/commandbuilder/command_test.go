package commandbuilder_test

import (
	"strings"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/internal/commandbuilder"
	_ "github.com/equinor/radix-operator/pkg/apis/test/initlogger"
	"github.com/stretchr/testify/assert"
)

func TestNewCommand_empty(t *testing.T) {
	assert.Empty(t, commandbuilder.NewCommand("").String())
}

func TestNewCommand_seperator(t *testing.T) {
	assert.Equal(t, "a b", commandbuilder.NewCommand("a").AddArgf("b").String())
}

func TestAdd1000Elements(t *testing.T) {
	a := commandbuilder.NewCommand("")

	for i := 0; i < 1000; i++ {
		a.AddArgf("---arg---")
	}

	actual := a.String()
	msgCount := strings.Count(actual, "---arg---")
	sepCount := strings.Count(actual, " ")

	assert.Equal(t, 1000, msgCount)
	assert.Equal(t, 999, sepCount)
}

func Test_JoinHasCorrectElements(t *testing.T) {
	a := commandbuilder.NewCommand("")

	a.AddArgf("Start")

	for i := 0; i < 10; i++ {
		a.AddArgf("Hei!")
	}

	a.AddArgf("Stop")

	actual := a.String()

	assert.True(t, strings.HasPrefix(actual, "Start"))
	assert.True(t, strings.HasSuffix(actual, "Stop"))
}

func Test_CommandList_JoinsCorrectly(t *testing.T) {
	cl := commandbuilder.NewCommandList().AddStrCmd("echo 'hello world'").AddCmd(commandbuilder.NewCommand("echo test"))
	assert.Equal(t, cl.String(), "echo 'hello world' && echo test")
}
