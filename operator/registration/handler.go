package registration

import (
	"context"

	"github.com/equinor/radix-operator/operator/common"
	"github.com/rs/zerolog/log"

	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

// handler handler for radix registrations
type handler struct {
	kubeclient  kubernetes.Interface
	kubeutil    *kube.Kube
	radixclient radixclient.Interface
	events      common.SyncedEventRecorder
	hasSynced   common.HasSynced
}

// NewHandler creates a handler which deals with RadixRegistration resources
func NewHandler(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	eventRecorder record.EventRecorder,
	hasSynced common.HasSynced) common.Handler {

	handler := &handler{
		kubeclient:  kubeclient,
		kubeutil:    kubeutil,
		radixclient: radixclient,
		events:      common.SyncedEventRecorder{EventRecorder: eventRecorder},
		hasSynced:   hasSynced,
	}

	return handler
}

// Sync Is created on sync of resource
func (t *handler) Sync(ctx context.Context, namespace, name string) error {
	registration, err := t.kubeutil.GetRegistration(ctx, name)
	if err != nil {
		// The Registration resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			log.Ctx(ctx).Info().Msgf("RadixRegistration %s in work queue no longer exists", name)
			return nil
		}

		return err
	}
	ctx = log.Ctx(ctx).With().Str("app_name", registration.Name).Logger().WithContext(ctx)

	syncRegistration := registration.DeepCopy()
	log.Ctx(ctx).Debug().Msgf("Sync registration %s", syncRegistration.Name)
	application, _ := application.NewApplication(t.kubeclient, t.kubeutil, t.radixclient, syncRegistration)
	err = application.OnSync(ctx)
	if err != nil {
		t.events.RecordFailedEvent(syncRegistration, err)
		return err
	}

	t.hasSynced(true)
	t.events.RecordSuccessEvent(syncRegistration)
	return nil
}
