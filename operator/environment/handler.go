package environment

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/networkpolicy"
	"github.com/rs/zerolog/log"

	"github.com/equinor/radix-operator/operator/common"
	"github.com/equinor/radix-operator/pkg/apis/environment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"

	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

// handler handler for radix environments
type handler struct {
	kubeclient  kubernetes.Interface
	kubeutil    *kube.Kube
	radixclient radixclient.Interface
	events      common.SyncEventRecorder
}

// NewHandler creates a handler for managing RadixEnvironment resources
func NewHandler(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	eventRecorder record.EventRecorder) common.Handler {

	handler := &handler{
		kubeclient:  kubeclient,
		kubeutil:    kubeutil,
		radixclient: radixclient,
		events:      common.NewSyncEventRecorder(eventRecorder),
	}

	return handler
}

// Sync is called by kubernetes after the Controller Enqueues a work-item
// and collects components and determines whether state must be reconciled.
func (t *handler) Sync(ctx context.Context, namespace, name string) error {
	envConfig, err := t.radixclient.RadixV1().RadixEnvironments().Get(ctx, name, meta.GetOptions{})
	if err != nil {
		// The Environment resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			log.Ctx(ctx).Info().Msgf("RadixEnvironment %s in work queue no longer exists", name)
			return nil
		}

		return err
	}
	ctx = log.Ctx(ctx).With().Str("app_name", envConfig.Spec.AppName).Logger().WithContext(ctx)

	syncEnvironment := envConfig.DeepCopy()
	log.Ctx(ctx).Debug().Msgf("Sync environment %s", syncEnvironment.Name)

	radixRegistration, err := t.kubeutil.GetRegistration(ctx, syncEnvironment.Spec.AppName)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		// The Registration resource may no longer exist, but we proceed to clear resources
		log.Ctx(ctx).Debug().Msgf("RadixRegistration %s no longer exists", syncEnvironment.Spec.AppName)
	}

	// get RA error is ignored because nil is accepted
	radixApplication, _ := t.radixclient.RadixV1().RadixApplications(utils.GetAppNamespace(syncEnvironment.Spec.AppName)).
		Get(ctx, syncEnvironment.Spec.AppName, meta.GetOptions{})

	nw := networkpolicy.NewNetworkPolicy(t.kubeclient, t.kubeutil, syncEnvironment.Spec.AppName)
	env := environment.NewEnvironment(t.kubeclient, t.kubeutil, t.radixclient, syncEnvironment, radixRegistration, radixApplication, &nw)
	err = env.OnSync(ctx)
	if err != nil {
		t.events.RecordSyncErrorEvent(syncEnvironment, err)
		return err
	}

	t.events.RecordSyncSuccessEvent(syncEnvironment)
	return nil
}
