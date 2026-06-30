package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// createHttpRouteAndNamespaceForTest creates a httproute and its namespace for testing
// appName: the name of the application (used as a base for the namespace and httproute names)
// Returns cleanup function
func createHttpRouteAndNamespaceForTest(t *testing.T, c client.Client, appName string) {
	appNamespace := appName + "-app"
	createNamespaceForTest(t, c, appName)
	hostnames := []gatewayapiv1.Hostname{
		"unique1.hostname.com",
		"unique2.hostname.com",
		"unique3.hostname.com",
		"*.wildcarddomain.com",
	}
	err := createHttpRoute(t, c, appName+"-httproute", appNamespace, hostnames, false)
	require.NoError(t, err)
}

// TestGatewayWebhookHttpRouteValidation tests that the webhook is working by verifying createHttpRouteUsableValidator
func TestGatewayWebhookHttpRouteValidation(t *testing.T) {
	c := getClient(t)
	appName := "test-httproute-validation"
	createHttpRouteAndNamespaceForTest(t, c, appName)

	t.Run("fails validation when route is not unique", func(t *testing.T) {
		appName := "test-new-httproute-validation-1"
		appNamespace := appName + "-app"
		nsCleanup := createNamespaceForTest(t, c, appName)
		defer nsCleanup()

		hostnames := []gatewayapiv1.Hostname{
			"unique4.hostname.com",
			"unique5.hostname.com",
			"unique3.hostname.com",
		}
		err := createHttpRoute(t, c, "uniqueroute", appNamespace, hostnames, true)

		assert.Error(t, err, "Should reject http route that is not unique outside its own namespace")
		if err != nil {
			t.Logf("Got expected error: %v", err)
		}
	})

	t.Run("succeeds validation when route is unique", func(t *testing.T) {
		appName := "test-new-httproute-validation-2"
		appNamespace := appName + "-app"
		nsCleanup := createNamespaceForTest(t, c, appName)
		defer nsCleanup()

		hostnames := []gatewayapiv1.Hostname{
			"unique4.hostname.com",
			"unique5.hostname.com",
			"unique6.hostname.com",
		}
		err := createHttpRoute(t, c, "uniqueroute", appNamespace, hostnames, true)

		assert.NoError(t, err, "Should accept http route that is unique outside its own namespace")
		if err != nil {
			t.Logf("Got expected error: %v", err)
		}
	})
}

func createHttpRoute(t *testing.T, c client.Client, name string, namespace string, hostnames []gatewayapiv1.Hostname, dryRunAll bool) error {
	pathMatchType := gatewayapiv1.PathMatchPathPrefix
	pathMatchValue := "/"
	var portNumber int32 = 8001

	hr := &gatewayapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: gatewayapiv1.HTTPRouteSpec{
			Hostnames: hostnames,
			Rules: []gatewayapiv1.HTTPRouteRule{
				{
					Matches: []gatewayapiv1.HTTPRouteMatch{
						{
							Path: &gatewayapiv1.HTTPPathMatch{
								Type: &pathMatchType, Value: &pathMatchValue,
							},
						},
					},
					BackendRefs: []gatewayapiv1.HTTPBackendRef{
						{
							BackendRef: gatewayapiv1.BackendRef{BackendObjectReference: gatewayapiv1.BackendObjectReference{Name: "web", Port: &portNumber}},
						},
					},
				},
			},
		},
	}

	if dryRunAll {
		return c.Create(t.Context(), hr, client.DryRunAll)
	}
	return c.Create(t.Context(), hr)
}
