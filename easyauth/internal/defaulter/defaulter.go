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
	reader         client.Reader
	annotation     string
	oauth2Template *radixv1.OAuth2
}

// NewRadixApplicationDefaulter returns a defaulter for RadixApplications with annotation, that sets the OAuth2
// configuration from oauth2Template (see NewOAuth2Template) on public components. reader reads the RadixRegistration
// of the application, to get the AppID used in the client ID. It should read from the API server and not a cache,
// since the RadixRegistration may have just been created.
func NewRadixApplicationDefaulter(reader client.Reader, annotation string, oauth2Template *radixv1.OAuth2) *RadixApplicationDefaulter {
	return &RadixApplicationDefaulter{reader: reader, annotation: annotation, oauth2Template: oauth2Template}
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
		component.Authentication.OAuth2 = OAuth2Config(d.oauth2Template, clientID)

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

// NewOAuth2Template returns the OAuth2 configuration for public components, without the client ID. It is a copy of
// oauth2 from the EasyAuth config, with the credentials and OIDC issuer URL that EasyAuth requires.
func NewOAuth2Template(oauth2 radixv1.OAuth2, issuerURL string) *radixv1.OAuth2 {
	template := oauth2.DeepCopy()
	template.ClientID = ""
	template.Credentials = radixv1.Secret
	if template.OIDC == nil {
		template.OIDC = &radixv1.OAuth2OIDC{}
	}
	template.OIDC.IssuerURL = issuerURL
	return template
}

// OAuth2Config returns the EasyAuth OAuth2 configuration for public components of an application
func OAuth2Config(template *radixv1.OAuth2, clientID string) *radixv1.OAuth2 {
	oauth2 := template.DeepCopy()
	oauth2.ClientID = clientID
	return oauth2
}

// IsPublic mirrors how the operator determines if a component is exposed to the internet
func IsPublic(component *radixv1.RadixComponent) bool {
	//nolint:staticcheck
	return component.PublicPort != "" || component.Public
}

var _ admission.Defaulter[*radixv1.RadixApplication] = &RadixApplicationDefaulter{}
