package commands

import (
	"path/filepath"
	"testing"

	"github.com/statoil/radix-operator/pkg/apis/kube"
	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateRole(t *testing.T) {
	file, _ := filepath.Abs("../mockdata/radixApplication.yaml")
	app := getRadixApplication(file)
	rolebinding := kube.BrigadeRole("an_id", app, metav1.OwnerReference{})

	assert.Equal(t, "radix-brigade-a_name-an_id", rolebinding.Name)
	assert.Equal(t, "Role", rolebinding.Kind)
}
