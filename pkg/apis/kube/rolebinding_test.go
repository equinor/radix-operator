package kube

import (
	"io/ioutil"
	"testing"

	"github.com/statoil/radix-operator/pkg/apis/radix/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/radix-operator/common"
	"github.com/stretchr/testify/assert"
	auth "k8s.io/api/rbac/v1"

	"k8s.io/client-go/kubernetes/fake"
)

func init() {
	log.SetOutput(ioutil.Discard)
}

func Test_Create_Rolebindings(t *testing.T) {
	radixApp, _ := common.GetRadixAppFromFile(sampleApp)
	kubeclient := fake.NewSimpleClientset()
	kubeutil, _ := New(kubeclient)

	t.Run("It creates rolebindings", func(t *testing.T) {
		err := kubeutil.ApplyRbacRadixApplication(radixApp)
		assert.NoError(t, err)
		assertRoleBindings(t, radixApp, kubeutil)
	})

	t.Run("It doesn't fail when re-running creation", func(t *testing.T) {
		err := kubeutil.ApplyRbacRadixApplication(radixApp)
		assert.NoError(t, err)
		assertRoleBindings(t, radixApp, kubeutil)
	})
}

func assertRoleBindings(t *testing.T, radixApp *v1.RadixApplication, kubeutil *Kube) {
	for _, env := range radixApp.Spec.Environments {
		for _, authConfig := range env.Authorization {
			rb, err := kubeutil.kubeClient.RbacV1().RoleBindings(fmt.Sprintf("%s-%s", radixApp.Name, env.Name)).Get(fmt.Sprintf("%s-%s", radixApp.Name, authConfig.Role), metav1.GetOptions{})
			assert.NoError(t, err)
			assert.Equal(t, authConfig.Role, rb.RoleRef.Name)
			assert.Equal(t, "ClusterRole", rb.RoleRef.Kind)
			assertGroups(t, authConfig.Groups, rb.Subjects)
		}
	}
}

func assertGroups(t *testing.T, configuredGroups []string, actualGroups []auth.Subject) {
	assert.Len(t, actualGroups, len(configuredGroups))
	for _, group := range configuredGroups {
		matchingGroup := false
		for _, subject := range actualGroups {
			assert.Equal(t, "Group", subject.Kind)
			if group == subject.Name {
				matchingGroup = true
			}
		}
		assert.True(t, matchingGroup, group)
	}
}

func TestCreateBrigadeRolebinding(t *testing.T) {
	rolebinding := BrigadeRoleBinding("app_name", "role_name", []string{"1g", "2g"}, metav1.OwnerReference{})

	assert.Equal(t, "role_name-binding", rolebinding.Name)
	assert.Equal(t, "RoleBinding", rolebinding.Kind)
}
