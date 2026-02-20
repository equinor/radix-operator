package e2e

import (
	"context"
	"fmt"
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
func createHttpRouteAndNamespaceForTest(t *testing.T, c client.Client, appName string) func() {
	appNamespace := appName + "-app"
	nsCleanup := createNamespaceForTest(t, c, appName)
	hostnames := []gatewayapiv1.Hostname{
		"unique1.hostname.com",
		"unique2.hostname.com",
		"unique3.hostname.com",
		"*.wildcarddomain.com",
	}
	hrCleanup, err := createHttpRoute(t, c, appName+"-httproute", appNamespace, hostnames, false)

	require.NoError(t, err)

	return func() {
		nsCleanup()
		hrCleanup()
	}
}

// TestGatewayWebhookHttpRouteValidation tests that the webhook is working by verifying createHttpRouteUsableValidator
func TestGatewayWebhookHttpRouteValidation(t *testing.T) {
	c := getClient(t)
	appName := "test-httproute-validation"
	cleanup := createHttpRouteAndNamespaceForTest(t, c, appName)
	defer cleanup()

	t.Run("fails validation when route is not unique", func(t *testing.T) {
		appName := "test-new-httproute-validation"
		appNamespace := appName + "-app"
		nsCleanup := createNamespaceForTest(t, c, appName)
		defer nsCleanup()

		hostnames := []gatewayapiv1.Hostname{
			"unique4.hostname.com",
			"unique5.hostname.com",
			"unique3.hostname.com",
		}
		_, err := createHttpRoute(t, c, "uniqueroute", appNamespace, hostnames, true)

		assert.Error(t, err, "Should reject http route that is not unique outside its own namespace")
		if err != nil {
			t.Logf("Got expected error: %v", err)
		}
	})

	t.Run("succeeds validation when route is unique", func(t *testing.T) {
		appName := "test-new-httproute-validation"
		appNamespace := appName + "-app"
		nsCleanup := createNamespaceForTest(t, c, appName)
		defer nsCleanup()

		hostnames := []gatewayapiv1.Hostname{
			"unique4.hostname.com",
			"unique5.hostname.com",
			"unique6.hostname.com",
		}
		_, err := createHttpRoute(t, c, "uniqueroute", appNamespace, hostnames, true)

		assert.NoError(t, err, "Should accept http route that is unique outside its own namespace")
		if err != nil {
			t.Logf("Got expected error: %v", err)
		}
	})

	t.Run("succeeds validation when updating route", func(t *testing.T) {
		hostnames := []gatewayapiv1.Hostname{
			"unique1.hostname.com",
			"unique2.hostname.com",
			"unique4.hostname.com",
		}
		appName := "test-httproute-validation"
		appNamespace := appName + "-app"
		routeName := appName + "-httproute"
		err := updateHttpRoute(t, c, routeName, appNamespace, hostnames, true)

		assert.NoError(t, err, "Should accept update of existing http route")
		if err != nil {
			t.Logf("Got expected error: %v", err)
		}
	})

	t.Run("fails validation when existing route has conflicting wildcard", func(t *testing.T) {
		appName := "test-new-httproute-validation"
		appNamespace := appName + "-app"
		nsCleanup := createNamespaceForTest(t, c, appName)
		defer nsCleanup()

		hostnames := []gatewayapiv1.Hostname{
			"unique4.hostname.com",
			"unique.wildcarddomain.com",
			"unique6.hostname.com",
		}
		_, err := createHttpRoute(t, c, "uniqueroute", appNamespace, hostnames, true)

		assert.Error(t, err, "Should reject http route that has a conflicting wildcard domain outside its own namespace")
		if err != nil {
			t.Logf("Got expected error: %v", err)
		}
	})

	t.Run("fails validation when incoming route has conflicting wildcard", func(t *testing.T) {
		appName := "test-new-httproute-validation"
		appNamespace := appName + "-app"
		nsCleanup := createNamespaceForTest(t, c, appName)
		defer nsCleanup()

		hostnames := []gatewayapiv1.Hostname{
			"unique1.test.com",
			"*.hostname.com",
			"unique2.test.com",
		}
		_, err := createHttpRoute(t, c, "uniqueroute", appNamespace, hostnames, true)

		assert.Error(t, err, "Should reject http route that has a conflicting wildcard domain outside its own namespace")
		if err != nil {
			t.Logf("Got expected error: %v", err)
		}
	})

	t.Run("fails validation when route is not unique even if casing is mixed", func(t *testing.T) {
		appName := "test-new-httproute-validation"
		appNamespace := appName + "-app"
		nsCleanup := createNamespaceForTest(t, c, appName)
		defer nsCleanup()

		hostnames := []gatewayapiv1.Hostname{
			"unique4.hostname.com",
			"uniQUE3.HOSTname.com",
			"UNIQUE2.hostNAME.com",
		}
		_, err := createHttpRoute(t, c, "uniqueroute", appNamespace, hostnames, true)

		assert.Error(t, err, "Should reject http route that is not unique outside its own namespace")
		if err != nil {
			t.Logf("Got expected error: %v", err)
		}
	})
}

func createHttpRoute(t *testing.T, c client.Client, name string, namespace string, hostnames []gatewayapiv1.Hostname, dryRunAll bool) (func(), error) {
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

	var err error
	if dryRunAll {
		err = c.Create(t.Context(), hr, client.DryRunAll)
	} else {
		err = c.Create(t.Context(), hr)
	}
	if err != nil {
		return nil, err
	}

	cleanup := func() {
		if dryRunAll == false {
			_ = c.Delete(context.Background(), hr)
		}
	}

	return cleanup, nil
}

func updateHttpRoute(t *testing.T, c client.Client, name string, namespace string, hostnames []gatewayapiv1.Hostname, dryRunAll bool) error {
	// Get the existing HTTPRoute from the cluster
	httpRoute := &gatewayapiv1.HTTPRoute{}
	err := c.Get(t.Context(), client.ObjectKey{Name: name, Namespace: namespace}, httpRoute)
	if err != nil {
		return fmt.Errorf("failed to get existing HTTPRoute: %w", err)
	}

	// Update the hostnames
	if hostnames != nil {
		httpRoute.Spec.Hostnames = hostnames
	}

	// Update with the current ResourceVersion
	if dryRunAll {
		err = c.Update(t.Context(), httpRoute, client.DryRunAll)
	} else {
		err = c.Update(t.Context(), httpRoute)
	}
	if err != nil {
		return err
	}

	return nil
}
