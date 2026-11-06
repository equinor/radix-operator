package defaulter

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/equinor/radix-operator/easyauth/internal/oauth"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

const Path = "/radix/v1/radixapplication/mutate"

// RadixApplicationDefaulter mutates RadixApplications that have the configured annotation
type RadixApplicationDefaulter struct {
	reader                      client.Reader
	annotation                  string
	allowedIdentitiesAnnotation string
	oauth2Template              *radixv1.OAuth2
}

// NewRadixApplicationDefaulter returns a defaulter for RadixApplications with annotation, that sets the OAuth2
// configuration from oauth2Template (see oauth.NewTemplate) on public components. reader reads the RadixRegistration
// of the application, to get the AppID used in the client ID. It should read from the API server and not a cache,
// since the RadixRegistration may have just been created. The OAuth2 configuration only allows the groups from
// allowedIdentitiesAnnotation, or oauth2Template.AllowedIdentities by default. The validating webhook rejects
// RadixApplications without allowed groups.
func NewRadixApplicationDefaulter(reader client.Reader, annotation, allowedIdentitiesAnnotation string, oauth2Template *radixv1.OAuth2) *RadixApplicationDefaulter {
	return &RadixApplicationDefaulter{reader: reader, annotation: annotation, allowedIdentitiesAnnotation: allowedIdentitiesAnnotation, oauth2Template: oauth2Template}
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

	clientID, err := oauth.ClientIDForApp(ctx, d.reader, obj.Name)
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
		component.Authentication.OAuth2 = oauth.ForApp(d.oauth2Template, clientID, oauth.SplitIdentities(obj.GetAnnotations()[d.allowedIdentitiesAnnotation]))

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

// IsPublic mirrors how the operator determines if a component is exposed to the internet
func IsPublic(component *radixv1.RadixComponent) bool {
	//nolint:staticcheck
	return component.PublicPort != "" || component.Public
}

var _ admission.Defaulter[*radixv1.RadixApplication] = &RadixApplicationDefaulter{}
