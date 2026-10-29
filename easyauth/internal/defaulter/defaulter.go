package defaulter

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

const Path = "/radix/v1/radixapplication/mutate"

// +kubebuilder:webhook:name=radixapplication.mutate.easyauth.radix.equinor.com,path=/radix/v1/radixapplication/mutate,mutating=true,failurePolicy=fail,sideEffects=None,groups=radix.equinor.com,resources=radixapplications,verbs=create;update,versions=v1,admissionReviewVersions={v1}

// RadixApplicationDefaulter mutates RadixApplications that have the configured Annotation
type RadixApplicationDefaulter struct {
	Annotation string
	// OIDC issuer URL set on the OAuth2 configuration of public components
	OAuth2IssuerURL string
}

func RegisterWithManager(mgr ctrl.Manager, d *RadixApplicationDefaulter) error {
	return ctrl.NewWebhookManagedBy(mgr, &radixv1.RadixApplication{}).
		WithDefaulter(d).
		WithDefaulterCustomPath(Path).
		Complete()
}

func (d *RadixApplicationDefaulter) Default(_ context.Context, obj *radixv1.RadixApplication) error {
	if _, ok := obj.GetAnnotations()[d.Annotation]; !ok {
		return nil
	}

	for i := range obj.Spec.Components {
		component := &obj.Spec.Components[i]
		if !IsPublic(component) {
			continue
		}

		if component.Authentication == nil {
			component.Authentication = &radixv1.Authentication{}
		}
		component.Authentication.OAuth2 = OAuth2Config(obj.Name, d.OAuth2IssuerURL)

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
func OAuth2Config(appName, issuerURL string) *radixv1.OAuth2 {
	return &radixv1.OAuth2{
		ClientID:               fmt.Sprintf("%s.dev.radix.equinor.com", appName),
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
