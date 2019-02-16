package application

const clusterName = "AnyClusterName"
const dnsZone = "dev.radix.equinor.com"
const containerRegistry = "any.container.registry"

/*
func setupTest() (*test.Utils, kube.Interface) {
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	registrationHandler := registration.NewRegistrationHandler(kubeclient, radixclient)
	applicationHandler := NewApplicationHandler(kubeclient, radixclient)

	handlerTestUtils := test.NewHandlerTestUtils(kubeclient, radixclient, &registrationHandler, &applicationHandler, nil)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)
	return &handlerTestUtils, kubeclient
}

func TestObjectCreatedUpdated_WithEnvironments_NamespacesAreCreated(t *testing.T) {
	handlerTestUtils, kubeclient := setupTest()

	handlerTestUtils.ApplyApplication(utils.ARadixApplication().
		WithAppName("any-app").
		WithEnvironment("dev", "master").
		WithEnvironment("prod", ""))

	t.Run("validate namespace creation", func(t *testing.T) {
		devNs, _ := kubeclient.CoreV1().Namespaces().Get("any-app-dev", metav1.GetOptions{})
		assert.NotNil(t, devNs)
		prodNs, _ := kubeclient.CoreV1().Namespaces().Get("any-app-prod", metav1.GetOptions{})
		assert.NotNil(t, prodNs)
	})

	t.Run("validate rolebindings", func(t *testing.T) {
		t.Parallel()
		rolebindings, _ := kubeclient.RbacV1().RoleBindings("any-app-dev").List(metav1.ListOptions{})
		assert.Equal(t, 1, len(rolebindings.Items), "Number of rolebindings was not expected")
		assert.Equal(t, "radix-app-admin-envs", rolebindings.Items[0].GetName(), "Expected rolebinding radix-app-admin-envs to be there by default")

		rolebindings, _ = kubeclient.RbacV1().RoleBindings("any-app-prod").List(metav1.ListOptions{})
		assert.Equal(t, 1, len(rolebindings.Items), "Number of rolebindings was not expected")
		assert.Equal(t, "radix-app-admin-envs", rolebindings.Items[0].GetName(), "Expected rolebinding radix-app-admin-envs to be there by default")
	})
}
*/
