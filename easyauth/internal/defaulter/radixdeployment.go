package defaulter

import (
	"context"
	"errors"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"

	admissionv1 "k8s.io/api/admission/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
)

const RadixDeploymentPath = "/radix/v1/radixdeployment/mutate"

// RadixDeploymentDefaulter copies the EasyAuth annotation from the RadixApplication to new RadixDeployments
type RadixDeploymentDefaulter struct {
	reader     client.Reader
	annotation string
}

// NewRadixDeploymentDefaulter returns a defaulter that, when a RadixDeployment is created, sets annotation with the
// value from the RadixApplication of the app, or removes it if the RadixApplication doesn't have it. reader should
// read from the API server and not a cache, since the RadixApplication may have just been applied by the pipeline.
func NewRadixDeploymentDefaulter(reader client.Reader, annotation string) *RadixDeploymentDefaulter {
	return &RadixDeploymentDefaulter{reader: reader, annotation: annotation}
}

// SetupWithManager registers the mutating webhook with the Manager
func (d *RadixDeploymentDefaulter) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &radixv1.RadixDeployment{}).
		WithDefaulter(d).
		WithDefaulterCustomPath(RadixDeploymentPath).
		Complete()
}

func (d *RadixDeploymentDefaulter) Default(ctx context.Context, obj *radixv1.RadixDeployment) error {
	// The annotation reflects the RadixApplication when the RadixDeployment was created, and is not changed afterwards
	if req, err := admission.RequestFromContext(ctx); err == nil && req.Operation != admissionv1.Create {
		return nil
	}

	appName, err := kube.GetAppNameFromNamespace(ctx, obj.Namespace, d.reader)
	if errors.Is(err, kube.ErrNamespaceHasNoAppLabel) {
		delete(obj.Annotations, d.annotation)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get app name for RadixDeployment %s/%s: %w", obj.Namespace, obj.Name, err)
	}

	ra := &radixv1.RadixApplication{Name: appName, Namespace: utils.GetAppNamespace(appName)}
	if err := d.reader.Get(ctx, client.ObjectKeyFromObject(ra), ra); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to get RadixApplication %s: %w", appName, err)
	}

	value, enabled := ra.GetAnnotations()[d.annotation]
	if !enabled {
		delete(obj.Annotations, d.annotation)
		return nil
	}

	if obj.Annotations == nil {
		obj.Annotations = map[string]string{}
	}
	obj.Annotations[d.annotation] = value
	return nil
}

var _ admission.Defaulter[*radixv1.RadixDeployment] = &RadixDeploymentDefaulter{}
