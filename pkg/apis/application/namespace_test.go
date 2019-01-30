package application

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/utils"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	log.SetOutput(ioutil.Discard)
}

func Test_Create_Radix_Environments(t *testing.T) {
	radixRegistration, _ := utils.GetRadixRegistrationFromFile(sampleRegistration)
	radixApp, _ := utils.GetRadixApplication(sampleApp)
	app := NewApplication(kubeclient, radixClient, radixRegistration)

	label := fmt.Sprintf("radixApp=%s", radixRegistration.Name)
	t.Run("It can create environments", func(t *testing.T) {
		for _, env := range radixApp.Spec.Environments {
			err := app.createAppNamespace()
			assert.NoError(t, err)
		}
		namespaces, _ := kubeclient.CoreV1().Namespaces().List(metav1.ListOptions{
			LabelSelector: label,
		})
		assert.Len(t, namespaces.Items, 2)
	})

	t.Run("It doesn't fail when re-running creation", func(t *testing.T) {
		for _, env := range radixApp.Spec.Environments {
			err := kubeutil.CreateEnvironment(radixRegistration, env.Name)
			assert.NoError(t, err)
		}
		namespaces, _ := kubeclient.CoreV1().Namespaces().List(metav1.ListOptions{
			LabelSelector: label,
		})
		assert.Len(t, namespaces.Items, 2)
	})
}
