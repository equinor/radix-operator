package environment

import (
	"context"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/networkpolicy"
	"github.com/rs/zerolog/log"

	"github.com/equinor/radix-operator/pkg/apis/environment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a Environment is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a Environment
	// is synced successfully
	MessageResourceSynced = "Radix Environment synced successfully"
)

// Handler Handler for radix environments
type Handler struct {
	kubeclient  kubernetes.Interface
	kubeutil    *kube.Kube
	radixclient radixclient.Interface
	hasSynced   common.HasSynced
}

// NewHandler creates a handler for managing RadixEnvironment resources
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

// Sync is called by kubernetes after the Controller Enqueues a work-item
// and collects components and determines whether state must be reconciled.
func (t *Handler) Sync(ctx context.Context, namespace, name string, eventRecorder record.EventRecorder) error {
	envConfig, err := t.radixclient.RadixV1().RadixEnvironments().Get(context.TODO(), name, meta.GetOptions{})
	if err != nil {
		// The Environment resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			log.Info().Msgf("RadixEnvironment %s in work queue no longer exists", name)
			return nil
		}

		return err
	}

	syncEnvironment := envConfig.DeepCopy()
	log.Debug().Msgf("Sync environment %s", syncEnvironment.Name)

	radixRegistration, err := t.kubeutil.GetRegistration(ctx, syncEnvironment.Spec.AppName)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		// The Registration resource may no longer exist, but we proceed to clear resources
		log.Debug().Msgf("RadixRegistration %s no longer exists", syncEnvironment.Spec.AppName)
	}

	// get RA error is ignored because nil is accepted
	radixApplication, _ := t.radixclient.RadixV1().RadixApplications(utils.GetAppNamespace(syncEnvironment.Spec.AppName)).
		Get(context.TODO(), syncEnvironment.Spec.AppName, meta.GetOptions{})

	nw, err := networkpolicy.NewNetworkPolicy(t.kubeclient, t.kubeutil, syncEnvironment.Spec.AppName)
	if err != nil {
		return err
	}

	env, err := environment.NewEnvironment(t.kubeclient, t.kubeutil, t.radixclient, syncEnvironment, radixRegistration, radixApplication, &nw)

	if err != nil {
		return err
	}

	err = env.OnSync(ctx, meta.NewTime(time.Now().UTC()))
	if err != nil {
		// TODO: should we record a Warning event when there is an error, similar to batch handler? Possibly do it in common.Controller?
		return err
	}

	t.hasSynced(true)
	eventRecorder.Event(env.GetConfig(), core.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}
