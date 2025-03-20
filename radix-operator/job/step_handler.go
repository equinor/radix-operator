package job

import (
	"context"
	"regexp"

	apiconfig "github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

const (
	jobStepPathPattern = `^status\.steps\{(?:name=)?([^{}]+[^=])\}$`
)

// StepHandler Job step stepHandler interface
type StepHandler interface {
	common.Handler
}

type stepHandler struct {
	kubeclient  kubernetes.Interface
	radixclient radixclient.Interface
	kubeutil    *kube.Kube
	hasSynced   common.HasSynced
	config      *apiconfig.Config
}

type stepEventHandlerOpts func(*stepHandler)

// NewStepHandler Constructor
func NewStepHandler(kubeclient kubernetes.Interface, kubeUtil *kube.Kube, radixClient radixclient.Interface, config *apiconfig.Config, hasSynced common.HasSynced, opts ...stepEventHandlerOpts) StepHandler {
	handler := stepHandler{
		kubeclient:  kubeclient,
		radixclient: radixClient,
		kubeutil:    kubeUtil,
		hasSynced:   hasSynced,
		config:      config,
	}
	for _, opt := range opts {
		opt(&handler)
	}
	return &handler
}

// Sync Is created on sync of resource
func (t *stepHandler) Sync(ctx context.Context, namespace, eventName string, eventRecorder record.EventRecorder) error {
	event, err := t.kubeclient.EventsV1().Events(namespace).Get(ctx, eventName, metav1.GetOptions{})
	if err != nil {
		// The Event resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			log.Ctx(ctx).Info().Msgf("Event %s/%s in work queue no longer exists", namespace, eventName)
			return nil
		}

		return err
	}
	if event.Regarding.Kind != radixv1.KindRadixJob {
		return nil
	}
	jobStepName, ok := extractStepName(event.Regarding.FieldPath)
	if !ok {
		return nil
	}
	log.Ctx(ctx).Debug().Msgf("Sync step event %s for the job %s, step %s", event.Name, event.Regarding.Name, jobStepName)
	radixJob, err := t.radixclient.RadixV1().RadixJobs(namespace).Get(ctx, event.Regarding.Name, metav1.GetOptions{})
	if err != nil {
		// The Job resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			log.Ctx(ctx).Info().Msgf("RadixJob %s/%s in work queue no longer exists", namespace, eventName)
			return nil
		}
		return err
	}

	ctx = log.Ctx(ctx).With().Str("app_name", radixJob.Spec.AppName).Logger().WithContext(ctx)

	syncJob := radixJob.DeepCopy()
	log.Ctx(ctx).Debug().Msgf("Sync job %s", syncJob.Name)

	//job := job.NewJob(t.kubeclient, t.kubeutil, t.radixclient, syncJob, t.config)
	//err = job.OnSync(ctx)
	//if err != nil {
	//	// TODO: should we record a Warning event when there is an error, similar to batch stepHandler? Possibly do it in common.Controller?
	//	// Put back on queue
	//	return err
	//}
	//
	//t.hasSynced(true)
	//eventRecorder.Event(syncJob, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func extractStepName(input string) (string, bool) {
	re := regexp.MustCompile(jobStepPathPattern)
	if match := re.FindStringSubmatch(input); match != nil {
		return match[1], true
	}
	return "", false
}
