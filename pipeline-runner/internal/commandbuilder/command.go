package commandbuilder

import (
	"fmt"
	"strings"

	"github.com/equinor/radix-common/utils/slice"
)

type CommandList struct{ cmds []*Command }

type Command struct {
	args []string
}

/* Command List */

func NewCommandList() *CommandList {
	return &CommandList{cmds: make([]*Command, 0)}
}

func (cl *CommandList) AddCmd(command *Command) *CommandList {
	cl.cmds = append(cl.cmds, command)
	return cl
}
func (cl *CommandList) AddStrCmd(format string, a ...any) *CommandList {
	cl.cmds = append(cl.cmds, NewCommand(format, a...))
	return cl
}

func (cl *CommandList) String() string {
	cmds := slice.Map(cl.cmds, func(c *Command) string { return c.String() })

	return strings.Join(cmds, " && ")
}

/* Command */

func NewCommand(formattedCmd string, a ...any) *Command {
	c := Command{}
	c.AddArgf(formattedCmd, a...)

	return &c
}

func (c *Command) AddArg(arg string) *Command {
	if arg == "" {
		return c
	}
	c.args = append(c.args, arg)
	return c
}

func (c *Command) AddArgf(format string, a ...any) *Command {
	if format == "" {
		return c
	}

	c.args = append(c.args, fmt.Sprintf(format, a...))
	return c
}

func (c *Command) String() string {
	return strings.Join(c.args, " ")
}
