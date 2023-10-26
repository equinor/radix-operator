package appender_test

import (
	"strings"
	"testing"

	"github.com/equinor/radix-operator/radix-operator/common/appender"
	"github.com/stretchr/testify/assert"
)

func TestNewContainer_empty(t *testing.T) {
	assert.Empty(t, appender.NewContainer().Join(" "))
}

func TestNewContainer_seperator(t *testing.T) {
	assert.Equal(t, "a OG b", appender.NewContainer().Addf("a").Addf("b").Join(" OG "))
}

func TestAdd1000Elements(t *testing.T) {
	a := appender.NewContainer()

	for i := 0; i < 1000; i++ {
		a.Addf("Hei!")
	}

	actual := a.Join(" :O ")
	msgCount := strings.Count(actual, "Hei!")
	sepCount := strings.Count(actual, ":O")

	assert.Equal(t, 1000, msgCount)
	assert.Equal(t, 999, sepCount)
}

func Test_JoinHasCorrectElements(t *testing.T) {
	a := appender.NewContainer()

	a.Addf("Start")

	for i := 0; i < 10; i++ {
		a.Addf("Hei!")
	}

	a.Addf("Stop")

	actual := a.Join(" :O ")

	assert.True(t, strings.HasPrefix(actual, "Start"))
	assert.True(t, strings.HasSuffix(actual, "Stop"))
}
