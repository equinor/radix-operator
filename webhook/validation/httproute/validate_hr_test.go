package httproute_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/webhook/validation/httproute"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func Test_Webhook_HttpRoute_ValidationSucceeds_WhenRouteIsNotUnique_ButInSameNamespace(t *testing.T) {
	validHttpRoute1 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")
	validHttpRoute2 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")

	client := test.CreateClient(radixNamespace(validHttpRoute1.Namespace), validHttpRoute1)

	validator := httproute.CreateOnlineValidator(client)
	wrns, err := validator.Validate(context.Background(), validHttpRoute2)
	assert.NoError(t, err)
	assert.Empty(t, wrns)
}

func Test_Webhook_HttpRoute_ValidationSkipped_WhenNamespace_IsNotManagedByRadix(t *testing.T) {
	// Namespace exists but has no kube.RadixAppLabel — treated as non-Radix namespace.
	nonRadixNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
	}
	httpRoute := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")
	httpRoute.Namespace = "default"

	client := test.CreateClient(nonRadixNs, httpRoute)

	validator := httproute.CreateOnlineValidator(client)
	wrns, err := validator.Validate(context.Background(), httpRoute)
	assert.NoError(t, err)
	assert.NotEmpty(t, wrns, "expected a warning when namespace is not managed by radix")
}

func Test_Webhook_HttpRoute_ValidationSucceeds_WhenExistingDomain_IsParentDomain_OfIncomingWildcardDomain(t *testing.T) {
	validHttpRoute1 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")
	validHttpRoute2 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")

	validHttpRoute2.Namespace = "someUniqueNamespace"
	validHttpRoute2.Spec.Hostnames = []gatewayapiv1.Hostname{
		"sub4.hostname.com",
		"*.sub1.hostname.com",
	}

	client := test.CreateClient(radixNamespace(validHttpRoute1.Namespace), radixNamespace(validHttpRoute2.Namespace), validHttpRoute1)

	validator := httproute.CreateOnlineValidator(client)
	wrns, err := validator.Validate(context.Background(), validHttpRoute2)
	assert.NoError(t, err)
	assert.Empty(t, wrns)
}

func Test_Webhook_HttpRoute_ValidationFails_WhenRoute_IsNot_Unique(t *testing.T) {
	validHttpRoute1 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")
	validHttpRoute2 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")

	validHttpRoute2.Namespace = "someUniqueNamespace"
	validHttpRoute2.Spec.Hostnames = []gatewayapiv1.Hostname{
		"sub4.hostname.com",
		"sub3.hostname.com",
		"sub2.hostname.com",
	}

	client := test.CreateClient(radixNamespace(validHttpRoute1.Namespace), radixNamespace(validHttpRoute2.Namespace), validHttpRoute1)

	validator := httproute.CreateOnlineValidator(client)
	_, err := validator.Validate(context.Background(), validHttpRoute2)
	assert.ErrorIs(t, err, httproute.ErrDuplicateHostname)
}

func Test_Webhook_HttpRoute_ValidationFails_WhenRoute_IsNot_Unique_EvenIf_MixedCasing(t *testing.T) {
	validHttpRoute1 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")
	validHttpRoute2 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")

	validHttpRoute2.Namespace = "someUniqueNamespace"
	validHttpRoute2.Spec.Hostnames = []gatewayapiv1.Hostname{
		"SUb4.hostname.com",
		"suB3.HOSTname.com",
		"sub2.hostNAME.com",
	}

	client := test.CreateClient(radixNamespace(validHttpRoute1.Namespace), radixNamespace(validHttpRoute2.Namespace), validHttpRoute1)

	validator := httproute.CreateOnlineValidator(client)
	_, err := validator.Validate(context.Background(), validHttpRoute2)
	assert.ErrorIs(t, err, httproute.ErrDuplicateHostname)
}

func Test_Webhook_HttpRoute_ValidationSucceeds_WhenRoute_Is_Unique(t *testing.T) {
	validHttpRoute1 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")
	validHttpRoute2 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")

	validHttpRoute2.Namespace = "someUniqueNamespace"
	validHttpRoute2.Spec.Hostnames = []gatewayapiv1.Hostname{
		"sub4.hostname.com",
		"sub5.hostname.com",
		"sub6.hostname.com",
	}

	client := test.CreateClient(radixNamespace(validHttpRoute1.Namespace), radixNamespace(validHttpRoute2.Namespace), validHttpRoute1)

	validator := httproute.CreateOnlineValidator(client)
	wrns, err := validator.Validate(context.Background(), validHttpRoute2)
	assert.NoError(t, err)
	assert.Empty(t, wrns)
}

func Test_Webhook_HttpRoute_ValidationSucceeds_WhenPatching_SameRoute(t *testing.T) {
	validHttpRoute := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")
	client := test.CreateClient(radixNamespace(validHttpRoute.Namespace), validHttpRoute)

	validator := httproute.CreateOnlineValidator(client)
	wrns, err := validator.Validate(context.Background(), validHttpRoute)
	assert.NoError(t, err)
	assert.Empty(t, wrns)
}

func Test_Webhook_HttpRoute_ValidationFails_WhenExistingRoute_HasWildcard(t *testing.T) {
	validHttpRoute1 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")
	validHttpRoute2 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")

	// Existing route
	validHttpRoute2.Namespace = "someUniqueNamespace"
	validHttpRoute1.Spec.Hostnames = []gatewayapiv1.Hostname{
		"*.hostname.com",
		"sub5.test.com",
		"sub6.test.com",
	}

	// Incoming route
	validHttpRoute2.Spec.Hostnames = []gatewayapiv1.Hostname{
		"sub1.hostname.com",
		"sub1.test.com",
		"sub2.test.com",
	}

	client := test.CreateClient(radixNamespace(validHttpRoute1.Namespace), radixNamespace(validHttpRoute2.Namespace), validHttpRoute1)

	validator := httproute.CreateOnlineValidator(client)
	_, err := validator.Validate(context.Background(), validHttpRoute2)
	assert.ErrorIs(t, err, httproute.ErrDuplicateHostname)
}

func Test_Webhook_HttpRoute_ValidationFails_WhenIncomingRoute_HasOverlappingWildcard(t *testing.T) {
	validHttpRoute1 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")
	validHttpRoute2 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")

	validHttpRoute2.Namespace = "someUniqueNamespace"
	validHttpRoute2.Spec.Hostnames = []gatewayapiv1.Hostname{
		"*.hostname.com",
		"sub5.test.com",
		"sub6.test.com",
	}

	client := test.CreateClient(radixNamespace(validHttpRoute1.Namespace), radixNamespace(validHttpRoute2.Namespace), validHttpRoute1)

	validator := httproute.CreateOnlineValidator(client)
	_, err := validator.Validate(context.Background(), validHttpRoute2)
	assert.ErrorIs(t, err, httproute.ErrDuplicateHostname)
}

func Test_Webhook_HttpRoute_ValidationFails_WhenIncomingRoute_HasOverlappingWildcard_OfMultilevelSubdomain(t *testing.T) {
	validHttpRoute1 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")
	validHttpRoute2 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")

	validHttpRoute2.Namespace = "someUniqueNamespace"
	validHttpRoute2.Spec.Hostnames = []gatewayapiv1.Hostname{
		"*.sub3.hostname.com",
		"sub5.test.com",
		"sub6.test.com",
	}

	client := test.CreateClient(radixNamespace(validHttpRoute1.Namespace), radixNamespace(validHttpRoute2.Namespace), validHttpRoute1)

	validator := httproute.CreateOnlineValidator(client)
	_, err := validator.Validate(context.Background(), validHttpRoute2)
	assert.ErrorIs(t, err, httproute.ErrDuplicateHostname)
}

func Test_Webhook_HttpRoute_ValidationSucceeds_WhenIncomingWildcardRoute_HasFewerSubdomains_ThanExistingRoute_WithSameParentDomain(t *testing.T) {
	validHttpRoute1 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")
	validHttpRoute2 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")

	validHttpRoute2.Namespace = "someUniqueNamespace"
	validHttpRoute2.Spec.Hostnames = []gatewayapiv1.Hostname{
		"*.sub3-3.sub3-2.sub3.hostname.com",
		"sub1.test.com",
		"sub2.test.com",
	}

	client := test.CreateClient(radixNamespace(validHttpRoute1.Namespace), radixNamespace(validHttpRoute2.Namespace), validHttpRoute1)

	validator := httproute.CreateOnlineValidator(client)
	wrns, err := validator.Validate(context.Background(), validHttpRoute2)
	assert.NoError(t, err)
	assert.Empty(t, wrns)
}

func Test_Webhook_HttpRoute_ValidationSucceeds_WhenBothRoutes_HaveWildcards_AtDifferentLevels(t *testing.T) {
	validHttpRoute1 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")
	validHttpRoute2 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")

	validHttpRoute2.Namespace = "someUniqueNamespace"
	validHttpRoute2.Spec.Hostnames = []gatewayapiv1.Hostname{
		"*.sub4-2.sub4.hostname.com",
		"sub1.test.com",
		"sub2.test.com",
	}

	client := test.CreateClient(radixNamespace(validHttpRoute1.Namespace), radixNamespace(validHttpRoute2.Namespace), validHttpRoute1)

	validator := httproute.CreateOnlineValidator(client)
	wrns, err := validator.Validate(context.Background(), validHttpRoute2)
	assert.NoError(t, err)
	assert.Empty(t, wrns)
}

// radixNamespace creates a Namespace with the kube.RadixAppLabel set, which is
// required for the HTTPRoute validator to consider the namespace as managed by Radix.
func radixNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{kube.RadixAppLabel: name},
		},
	}
}
