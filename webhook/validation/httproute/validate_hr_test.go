package httproute_test

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/equinor/radix-operator/webhook/validation/httproute"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/yaml"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(gatewayapiv1.Install(scheme))
}

func Test_Webhook_HttpRoute_ValidationSucceeds_WhenExistingDomain_IsParentDomain_OfIncomingWildcardDomain(t *testing.T) {
	validHttpRoute1 := createValidHttpRoute(t)
	validHttpRoute2 := createValidHttpRoute(t)

	validHttpRoute2.Namespace = "someUniqueNamespace"
	validHttpRoute2.Spec.Hostnames = []gatewayapiv1.Hostname{
		"unique4.hostname.com",
		"*.unique1.hostname.com",
	}

	client := createClient(validHttpRoute1)

	validator := httproute.CreateOnlineValidator(client)
	wrns, err := validator.Validate(context.Background(), validHttpRoute2)
	assert.NoError(t, err)
	assert.Empty(t, wrns)
}

func Test_Webhook_HttpRoute_ValidationFails_WhenExistingDomain_Overlaps_WithIncomingWildcardDomain(t *testing.T) {
	validHttpRoute1 := createValidHttpRoute(t)
	validHttpRoute2 := createValidHttpRoute(t)

	validHttpRoute2.Namespace = "someUniqueNamespace"
	validHttpRoute2.Spec.Hostnames = []gatewayapiv1.Hostname{
		"unique4.hostname.com",
		"*.unique3.hostname.com",
	}

	client := createClient(validHttpRoute1)

	validator := httproute.CreateOnlineValidator(client)
	_, err := validator.Validate(context.Background(), validHttpRoute2)
	assert.ErrorIs(t, err, httproute.ErrDuplicateHostname)
}

func Test_Webhook_HttpRoute_ValidationFails_WhenRoute_IsNot_Unique(t *testing.T) {
	validHttpRoute1 := createValidHttpRoute(t)
	validHttpRoute2 := createValidHttpRoute(t)

	validHttpRoute2.Namespace = "someUniqueNamespace"
	validHttpRoute2.Spec.Hostnames = []gatewayapiv1.Hostname{
		"unique4.hostname.com",
		"unique3.hostname.com",
		"unique2.hostname.com",
	}

	client := createClient(validHttpRoute1)

	validator := httproute.CreateOnlineValidator(client)
	_, err := validator.Validate(context.Background(), validHttpRoute2)
	assert.ErrorIs(t, err, httproute.ErrDuplicateHostname)
}

func Test_Webhook_HttpRoute_ValidationFails_WhenRoute_IsNot_Unique_EvenIf_MixedCasing(t *testing.T) {
	validHttpRoute1 := createValidHttpRoute(t)
	validHttpRoute2 := createValidHttpRoute(t)

	validHttpRoute2.Namespace = "someUniqueNamespace"
	validHttpRoute2.Spec.Hostnames = []gatewayapiv1.Hostname{
		"unique4.hostname.com",
		"uniQUE3.HOSTname.com",
		"UNIQUE2.hostNAME.com",
	}

	client := createClient(validHttpRoute1)

	validator := httproute.CreateOnlineValidator(client)
	_, err := validator.Validate(context.Background(), validHttpRoute2)
	assert.ErrorIs(t, err, httproute.ErrDuplicateHostname)
}

func Test_Webhook_HttpRoute_ValidationSucceeds_WhenRoute_Is_Unique(t *testing.T) {
	validHttpRoute1 := createValidHttpRoute(t)
	validHttpRoute2 := createValidHttpRoute(t)

	validHttpRoute2.Namespace = "someUniqueNamespace"
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

func Test_Webhook_HttpRoute_ValidationSucceeds_WhenPatching_SameRoute(t *testing.T) {
	validHttpRoute := createValidHttpRoute(t)
	client := createClient(validHttpRoute)

	validator := httproute.CreateOnlineValidator(client)
	wrns, err := validator.Validate(context.Background(), validHttpRoute)
	assert.NoError(t, err)
	assert.Empty(t, wrns)
}

func Test_Webhook_HttpRoute_ValidationFails_WhenExistingRoute_HasWildcard(t *testing.T) {
	validHttpRoute1 := createValidHttpRoute(t)
	validHttpRoute2 := createValidHttpRoute(t)

	// Existing route
	validHttpRoute2.Namespace = "someUniqueNamespace"
	validHttpRoute1.Spec.Hostnames = []gatewayapiv1.Hostname{
		"*.hostname.com",
		"unique5.test.com",
		"unique6.test.com",
	}

	// Incoming route
	validHttpRoute2.Spec.Hostnames = []gatewayapiv1.Hostname{
		"unique1.hostname.com",
		"unique7.test.com",
		"unique8.test.com",
	}

	client := createClient(validHttpRoute1)

	validator := httproute.CreateOnlineValidator(client)
	_, err := validator.Validate(context.Background(), validHttpRoute2)
	assert.ErrorIs(t, err, httproute.ErrDuplicateHostname)
}

func Test_Webhook_HttpRoute_ValidationFails_WhenIncomingRoute_HasWildcard(t *testing.T) {
	validHttpRoute1 := createValidHttpRoute(t)
	validHttpRoute2 := createValidHttpRoute(t)

	validHttpRoute2.Namespace = "someUniqueNamespace"
	validHttpRoute2.Spec.Hostnames = []gatewayapiv1.Hostname{
		"*.hostname.com",
		"unique5.test.com",
		"unique6.test.com",
	}

	client := createClient(validHttpRoute1)

	validator := httproute.CreateOnlineValidator(client)
	_, err := validator.Validate(context.Background(), validHttpRoute2)
	assert.ErrorIs(t, err, httproute.ErrDuplicateHostname)
}

func createValidHttpRoute(t *testing.T) *gatewayapiv1.HTTPRoute {
	validHttpRoute := load[*gatewayapiv1.HTTPRoute]("./testdata/httproute.yaml", t)

	return validHttpRoute
}

func createClient(initObjs ...client.Object) client.Client {
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).Build()
}

func load[T client.Object](filename string, t *testing.T) T {
	raw := struct {
		metav1.TypeMeta   `json:",inline"`
		metav1.ObjectMeta `json:"metadata,omitempty"`
	}{}

	configFileContent, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}

	// Important: Must use sigs.k8s.io/yaml decoder to correctly unmarshal Kubernetes objects.
	// This package supports encoding and decoding of yaml for CRD struct types using the json tag.
	// The gopkg.in/yaml.v3 package requires the yaml tag.
	err = yaml.Unmarshal(configFileContent, &raw)
	if err != nil {
		t.Fatal(err)
	}

	gvk := raw.GetObjectKind().GroupVersionKind()
	tp, ok := scheme.AllKnownTypes()[gvk]
	if !ok {
		panic(fmt.Sprintf("scheme does not know GroupVersionKind %s", gvk.String()))
	}

	obj := reflect.New(tp)
	objP := obj.Interface()
	err = yaml.Unmarshal(configFileContent, objP)
	if err != nil {
		t.Fatal(err)
	}

	return objP.(T)
}
