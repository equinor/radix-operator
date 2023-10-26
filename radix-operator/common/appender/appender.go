package appender

import (
	"fmt"
	"strings"
)

type Container struct {
	strs []string
}

func NewContainer() *Container {
	return &Container{
		strs: make([]string, 0),
	}
}

func (b *Container) Addf(format string, a ...any) *Container {

	b.strs = append(b.strs, fmt.Sprintf(format, a...))

	return b
}

func (b *Container) Join(seperator string) string {
	return strings.Join(b.strs, seperator)
}
