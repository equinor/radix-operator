package httproute_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/webhook/validation/httproute"
	"github.com/stretchr/testify/assert"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func Test_Webhook_HttpRoute_ValidationSucceeds_WhenRouteIsNotUnique_ButInSameNamespace(t *testing.T) {
	validHttpRoute1 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")
	validHttpRoute2 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")

	client := test.CreateClient(validHttpRoute1)

	validator := httproute.CreateOnlineValidator(client)
	wrns, err := validator.Validate(context.Background(), validHttpRoute2)
	assert.NoError(t, err)
	assert.Empty(t, wrns)
}

func Test_Webhook_HttpRoute_ValidationSucceeds_WhenExistingDomain_IsParentDomain_OfIncomingWildcardDomain(t *testing.T) {
	validHttpRoute1 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")
	validHttpRoute2 := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")

	validHttpRoute2.Namespace = "someUniqueNamespace"
	validHttpRoute2.Spec.Hostnames = []gatewayapiv1.Hostname{
		"sub4.hostname.com",
		"*.sub1.hostname.com",
	}

	client := test.CreateClient(validHttpRoute1)

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

	client := test.CreateClient(validHttpRoute1)

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

	client := test.CreateClient(validHttpRoute1)

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

	client := test.CreateClient(validHttpRoute1)

	validator := httproute.CreateOnlineValidator(client)
	wrns, err := validator.Validate(context.Background(), validHttpRoute2)
	assert.NoError(t, err)
	assert.Empty(t, wrns)
}

func Test_Webhook_HttpRoute_ValidationSucceeds_WhenPatching_SameRoute(t *testing.T) {
	validHttpRoute := test.Load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml")
	client := test.CreateClient(validHttpRoute)

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

	client := test.CreateClient(validHttpRoute1)

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

	client := test.CreateClient(validHttpRoute1)

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

	client := test.CreateClient(validHttpRoute1)

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

	client := test.CreateClient(validHttpRoute1)

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

	client := test.CreateClient(validHttpRoute1)

	validator := httproute.CreateOnlineValidator(client)
	wrns, err := validator.Validate(context.Background(), validHttpRoute2)
	assert.NoError(t, err)
	assert.Empty(t, wrns)
}
