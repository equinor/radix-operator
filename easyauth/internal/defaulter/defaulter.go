package defaulter

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/equinor/radix-operator/easyauth/internal/dex"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

const Path = "/radix/v1/radixapplication/mutate"

// +kubebuilder:webhook:name=radixapplication.mutate.easyauth.radix.equinor.com,path=/radix/v1/radixapplication/mutate,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,reinvocationPolicy=IfNeeded,sideEffects=None,servicePort=443,groups=radix.equinor.com,resources=radixapplications,verbs=create;update,versions=v1,admissionReviewVersions={v1}

// RadixApplicationDefaulter mutates RadixApplications that have the configured annotation
type RadixApplicationDefaulter struct {
	reader          client.Reader
	annotation      string
	oauth2IssuerURL string
}

// NewRadixApplicationDefaulter returns a defaulter for RadixApplications with annotation, that sets oauth2IssuerURL on
// public components. reader reads the RadixRegistration of the application, to get the AppID used in the client ID.
// It should read from the API server and not a cache, since the RadixRegistration may have just been created.
func NewRadixApplicationDefaulter(reader client.Reader, annotation, oauth2IssuerURL string) *RadixApplicationDefaulter {
	return &RadixApplicationDefaulter{reader: reader, annotation: annotation, oauth2IssuerURL: oauth2IssuerURL}
}

// SetupWithManager registers the mutating webhook with the Manager
func (d *RadixApplicationDefaulter) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &radixv1.RadixApplication{}).
		WithDefaulter(d).
		WithDefaulterCustomPath(Path).
		Complete()
}

func (d *RadixApplicationDefaulter) Default(ctx context.Context, obj *radixv1.RadixApplication) error {
	if _, ok := obj.GetAnnotations()[d.annotation]; !ok {
		return nil
	}

	clientID, err := dex.ClientIDForApp(ctx, d.reader, obj.Name)
	if err != nil {
		return err
	}

	for i := range obj.Spec.Components {
		component := &obj.Spec.Components[i]
		if !IsPublic(component) {
			continue
		}

		if component.Authentication == nil {
			component.Authentication = &radixv1.Authentication{}
		}
		component.Authentication.OAuth2 = OAuth2Config(clientID, d.oauth2IssuerURL)

		// Environment specific OAuth2 config is merged on top of the component config, so it must be
		// removed for the component config to take effect
		for j := range component.EnvironmentConfig {
			if envAuth := component.EnvironmentConfig[j].Authentication; envAuth != nil {
				envAuth.OAuth2 = nil
			}
		}
	}

	return nil
}

// OAuth2Config returns the EasyAuth OAuth2 configuration for public components of an application
func OAuth2Config(clientID, issuerURL string) *radixv1.OAuth2 {
	return &radixv1.OAuth2{
		ClientID:               clientID,
		Credentials:            radixv1.Secret,
		Scope:                  "openid profile email offline_access",
		SetAuthorizationHeader: new(false),
		SetXAuthRequestHeaders: new(false),
		OIDC: &radixv1.OAuth2OIDC{
			IssuerURL: issuerURL,
		},
	}
}

// IsPublic mirrors how the operator determines if a component is exposed to the internet
func IsPublic(component *radixv1.RadixComponent) bool {
	//nolint:staticcheck
	return component.PublicPort != "" || component.Public
}

var _ admission.Defaulter[*radixv1.RadixApplication] = &RadixApplicationDefaulter{}
