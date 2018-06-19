package brigade

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/statoil/radix-operator/radix-operator/common"

	log "github.com/Sirupsen/logrus"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
)

const (
	sampleRegistration = "../../../radix-operator/registration/testdata/sampleregistration.yaml"
)

func init() {
	log.SetOutput(ioutil.Discard)
}

func Test_BrigadeGateway_Can_Create_Projects(t *testing.T) {
	radixRegistration, _ := common.GetRadixRegistrationFromFile(sampleRegistration)
	secretCreated, secretUpdated := false, false

	nameHash := fmt.Sprintf("brigade-%s", shortSHA(projectPrefix+radixRegistration.Name))
	fakeClient := fake.NewSimpleClientset()

	reactorFunc := func(action core.Action) (bool, runtime.Object, error) {
		switch a := action.(type) {
		case core.CreateAction:
			createdApp, ok := a.GetObject().(*corev1.Secret)
			if ok && createdApp.Name == nameHash {
				if a.GetVerb() == "create" {
					secretCreated = true
				} else if a.GetVerb() == "update" {
					secretUpdated = true
				}
			}
		default:
			return false, nil, nil
		}

		return false, nil, nil
	}

	fakeClient.PrependReactor("create", "secrets", reactorFunc)
	fakeClient.PrependReactor("update", "secrets", reactorFunc)

	gateway, _ := New(fakeClient)

	t.Run("It creates a project", func(t *testing.T) {
		err := gateway.EnsureProject(radixRegistration)
		assert.NoError(t, err)

		wait.Poll(100*time.Millisecond, wait.ForeverTestTimeout, func() (bool, error) {
			return secretCreated, nil
		})

		brigadeProject, err := fakeClient.CoreV1().Secrets("default").Get(nameHash, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, brigadeProject)
	})

	t.Run("It updates a project", func(t *testing.T) {
		radixRegistration.Spec.DefaultScriptName = "testScript"
		err := gateway.EnsureProject(radixRegistration)
		assert.NoError(t, err)

		wait.Poll(100*time.Millisecond, wait.ForeverTestTimeout, func() (bool, error) {
			return secretUpdated, nil
		})

		brigadeProject, err := fakeClient.CoreV1().Secrets("default").Get(nameHash, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.EqualValues(t, "testScript", brigadeProject.StringData["defaultScriptName"])
	})
}
