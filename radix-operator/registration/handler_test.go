package registration

import (
	"io/ioutil"

	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetOutput(ioutil.Discard)
}

/*
func Test_RadixRegistrationHandler(t *testing.T) {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "radix-docker",
			Namespace: "default",
		},
		Data: map[string][]byte{
			".dockerconfigjson": []byte("{\"auths\":{\"radixdev.azurecr.io\":{\"username\":\"testuser\",\"password\":\"mysecretpassword\",\"email\":\"frode.hus@outlook.com\",\"auth\":\"YXNkZjpxd2VydHk=\"}}}"),
		},
	}

	client := fake.NewSimpleClientset(secret)
	radixClient := fakeradix.NewSimpleClientset()
	registration, _ := utils.GetRadixRegistrationFromFile("testdata/sampleregistration.yaml")
	handler := NewRegistrationHandler(client, radixClient)
	radixClient.PrependReactor("get", "radixregistrations", func(action kubetest.Action) (handled bool, ret runtime.Object, err error) {
		return true, registration, nil
	})

	t.Run("It creates a registration", func(t *testing.T) {
		err := handler.ObjectCreated(registration)
		assert.NoError(t, err)
		ns, err := client.CoreV1().Namespaces().Get(utils.GetAppNamespace(registration.Name), metav1.GetOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, ns)
	})
	t.Run("It updates a registration", func(t *testing.T) {
		err := handler.ObjectUpdated(nil, registration)
		assert.NoError(t, err)
	})
}
*/
