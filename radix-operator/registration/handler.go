package registration

import (
	"context"

	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/rs/zerolog/log"

	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a Registration is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a Registration
	// is synced successfully
	MessageResourceSynced = "Radix Registration synced successfully"
)

// Handler Handler for radix registrations
type Handler struct {
	kubeclient  kubernetes.Interface
	kubeutil    *kube.Kube
	radixclient radixclient.Interface
	hasSynced   common.HasSynced
}

// NewHandler creates a handler which deals with RadixRegistration resources
func NewHandler(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	hasSynced common.HasSynced) Handler {

	handler := Handler{
		kubeclient:  kubeclient,
		kubeutil:    kubeutil,
		radixclient: radixclient,
		hasSynced:   hasSynced,
	}

	return handler
}

// Sync Is created on sync of resource
func (t *Handler) Sync(ctx context.Context, namespace, name string, eventRecorder record.EventRecorder) error {
	registration, err := t.kubeutil.GetRegistration(name)
	if err != nil {
		// The Registration resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			log.Info().Msgf("RadixRegistration %s in work queue no longer exists", name)
			return nil
		}

		return err
	}

	syncRegistration := registration.DeepCopy()
	log.Debug().Msgf("Sync registration %s", syncRegistration.Name)
	application, _ := application.NewApplication(t.kubeclient, t.kubeutil, t.radixclient, syncRegistration)
	err = application.OnSync(ctx)
	if err != nil {
		// TODO: should we record a Warning event when there is an error, similar to batch handler? Possibly do it in common.Controller?
		// Put back on queue.
		return err
	}

	t.hasSynced(true)
	eventRecorder.Event(syncRegistration, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}
