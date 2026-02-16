package httproute_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/webhook/validation/httproute"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(gatewayapiv1.Install(scheme))
}

func Test_Webhook_HttpRoute_ValidationFails_WhenRoute_IsNot_Unique(t *testing.T) {
	validHttpRoute1 := createValidHttpRoute()
	validHttpRoute2 := createValidHttpRoute()

	validHttpRoute2.Spec.Hostnames = []gatewayapiv1.Hostname{
		"unique4.hostname.com",
		"unique3.hostname.com",
		"unique2.hostname.com",
	}

	client := createClient(validHttpRoute1)

	validator := httproute.CreateOnlineValidator(client)
	_, err := validator.Validate(context.Background(), validHttpRoute2)
	assert.Error(t, err)
}

func Test_Webhook_HttpRoute_ValidationSucceeds_WhenRoute_Is_Unique(t *testing.T) {
	validHttpRoute1 := createValidHttpRoute()
	validHttpRoute2 := createValidHttpRoute()

	validHttpRoute2.Spec.Hostnames = []gatewayapiv1.Hostname{
		"unique4.hostname.com",
		"unique5.hostname.com",
		"unique6.hostname.com",
	}

	client := createClient(validHttpRoute1)

	validator := httproute.CreateOnlineValidator(client)
	wrns, err := validator.Validate(context.Background(), validHttpRoute2)
	assert.NoError(t, err)
	assert.Empty(t, wrns)
}

func createValidHttpRoute() *gatewayapiv1.HTTPRoute {
	validHttpRoute, _ := utils.GetHttpRouteFromFile("testdata/httproute.yaml")
	return validHttpRoute
}

func createClient(initObjs ...client.Object) client.Client {
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).Build()
}
