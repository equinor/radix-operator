package commands

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetRadixApplicationFromFile(t *testing.T) {
	file, _ := filepath.Abs("../mockdata/radixApplication.yaml")
	app := getRadixApplication(file)

	assert.Equal(t, "a_name", app.Name)
	assert.Equal(t, "RadixApplication", app.Kind)
	assert.Equal(t, "frontend", app.Spec.Components[0].Name)
	assert.Equal(t, "prod", app.Spec.Environments[1].Name)
}
