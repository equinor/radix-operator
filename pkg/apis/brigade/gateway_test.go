package brigade

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"

	radixv1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
)

var radixApp = &radixv1.RadixApplication{
	ObjectMeta: metav1.ObjectMeta{
		Name: "testapp",
	},
	Spec: radixv1.RadixApplicationSpec{
		Secrets: radixv1.SecretsMap{
			"test": "123",
		},
	},
}

func init() {
	log.SetOutput(ioutil.Discard)
}

func Test_BrigadeGateway_Can_Create_Projects(t *testing.T) {
	secretCreated := false
	nameHash := fmt.Sprintf("brigade-%s", shortSHA(projectPrefix+radixApp.Name))
	fakeClient := fake.NewSimpleClientset()

	reactorFunc := func(action core.Action) (bool, runtime.Object, error) {
		switch a := action.(type) {
		case core.CreateAction:
			createdApp, ok := a.GetObject().(*corev1.Secret)
			if ok && createdApp.Name == nameHash {
				secretCreated = true
			}
		default:
			return false, nil, nil
		}

		return false, nil, nil
	}

	fakeClient.PrependReactor("create", "secrets", reactorFunc)

	gateway := BrigadeGateway{
		client: fakeClient,
	}

	t.Run("Create project", func(t *testing.T) {
		err := gateway.EnsureProject(radixApp)
		assert.NoError(t, err)

		wait.Poll(100*time.Millisecond, wait.ForeverTestTimeout, func() (bool, error) {
			return secretCreated, nil
		})

		brigadeProject, err := fakeClient.CoreV1().Secrets("default").Get(nameHash, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, brigadeProject)
	})
}

func Test_BrigadeGateway_Fails_Without_Client(t *testing.T) {
	gateway := BrigadeGateway{
		client: nil,
	}

	err := gateway.EnsureProject(radixApp)
	assert.Error(t, err)
}
