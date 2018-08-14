package commands

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateRole(t *testing.T) {
	file, _ := filepath.Abs("../mockdata/radixApplication.yaml")
	app := getRadixApplication(file)
	rolebinding := createRole("an_id", app, metav1.OwnerReference{})

	assert.Equal(t, "radix-brigade-a_name-an_id", rolebinding.Name)
	assert.Equal(t, "Role", rolebinding.Kind)
}

func TestCreateRolebinding(t *testing.T) {
	rolebinding := createRolebinding("app_name", "role_name", []string{"1g", "2g"}, metav1.OwnerReference{})

	assert.Equal(t, "role_name-binding", rolebinding.Name)
	assert.Equal(t, "RoleBinding", rolebinding.Kind)
}
