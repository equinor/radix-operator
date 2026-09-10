package application

import (
	"context"

	"github.com/equinor/radix-operator/operator/common"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

// handler Instance variables
type handler struct {
	kubeclient  kubernetes.Interface
	radixclient radixclient.Interface
	kubeutil    *kube.Kube
	events      common.SyncEventRecorder
}

// NewHandler Constructor
func NewHandler(kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	eventRecorder record.EventRecorder) common.Handler {

	handler := &handler{
		kubeclient:  kubeclient,
		radixclient: radixclient,
		kubeutil:    kubeutil,
		events:      common.NewSyncEventRecorder(eventRecorder),
	}

	return handler
}

// Sync Is created on sync of resource
func (t *handler) Sync(ctx context.Context, namespace, name string) error {
	radixApplication, err := t.radixclient.RadixV1().RadixApplications(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		// The Application resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			log.Ctx(ctx).Info().Msgf("RadixApplication %s/%s in work queue no longer exists", namespace, name)
			return nil
		}

		return err
	}
	ctx = log.Ctx(ctx).With().Str("app_name", name).Logger().WithContext(ctx)

	radixRegistration, err := t.kubeutil.GetRegistration(ctx, radixApplication.Name)
	if err != nil {
		// The Registration resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			log.Ctx(ctx).Debug().Msgf("RadixRegistration %s no longer exists", radixApplication.Name)
			return nil
		}

		return err
	}

	syncApplication := radixApplication.DeepCopy()
	log.Ctx(ctx).Debug().Msgf("Sync application %s", syncApplication.Name)
	applicationConfig := application.NewApplicationConfig(t.kubeclient, t.kubeutil, t.radixclient, radixRegistration, radixApplication)
	err = applicationConfig.OnSync(ctx)
	if err != nil {
		t.events.RecordSyncErrorEvent(syncApplication, err)
		return err
	}

	t.events.RecordSyncSuccessEvent(syncApplication)
	return nil
}
