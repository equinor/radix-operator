package application

import (
	"context"

	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a Application Config is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a Application Config
	// is synced successfully
	MessageResourceSynced = "Radix Application synced successfully"
)

// Handler Instance variables
type Handler struct {
	kubeclient  kubernetes.Interface
	radixclient radixclient.Interface
	kubeutil    *kube.Kube
	hasSynced   common.HasSynced
	dnsConfig   *dnsalias.DNSConfig
}

// NewHandler Constructor
func NewHandler(kubeclient kubernetes.Interface, kubeutil *kube.Kube, radixclient radixclient.Interface, dnsConfig *dnsalias.DNSConfig, hasSynced common.HasSynced) Handler {

	handler := Handler{
		kubeclient:  kubeclient,
		radixclient: radixclient,
		kubeutil:    kubeutil,
		hasSynced:   hasSynced,
		dnsConfig:   dnsConfig,
	}

	return handler
}

// Sync Is created on sync of resource
func (t *Handler) Sync(ctx context.Context, namespace, name string, eventRecorder record.EventRecorder) error {
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
	applicationConfig := application.NewApplicationConfig(t.kubeclient, t.kubeutil, t.radixclient, radixRegistration, radixApplication, t.dnsConfig)
	err = applicationConfig.OnSync(ctx)
	if err != nil {
		// TODO: should we record a Warning event when there is an error, similar to batch handler? Possibly do it in common.Controller?
		// Put back on queue
		return err
	}

	t.hasSynced(true)
	eventRecorder.Event(syncApplication, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}
