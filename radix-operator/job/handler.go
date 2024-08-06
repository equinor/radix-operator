package job

import (
	"context"
	"time"

	apiconfig "github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
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
	// SuccessSynced is used as part of the Event 'reason' when a Job is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a Job
	// is synced successfully
	MessageResourceSynced = "Radix Job synced successfully"
)

// Handler Common handler interface
type Handler interface {
	common.Handler
	// CleanupJobHistory Cleanup the pipeline job history
	CleanupJobHistory(ctx context.Context, radixJob *v1.RadixJob)
}

type handler struct {
	kubeclient  kubernetes.Interface
	radixclient radixclient.Interface
	kubeutil    *kube.Kube
	hasSynced   common.HasSynced
	config      *apiconfig.Config
	jobHistory  job.History
}

type handlerOpts func(*handler)

// NewHandler Constructor
func NewHandler(kubeclient kubernetes.Interface, kubeUtil *kube.Kube, radixClient radixclient.Interface, config *apiconfig.Config, hasSynced common.HasSynced, opts ...handlerOpts) Handler {
	handler := handler{
		kubeclient:  kubeclient,
		radixclient: radixClient,
		kubeutil:    kubeUtil,
		hasSynced:   hasSynced,
		config:      config,
		jobHistory:  job.NewHistory(radixClient, kubeUtil, config.PipelineJobConfig.PipelineJobsHistoryLimit, config.PipelineJobConfig.PipelineJobsHistoryPeriodLimit),
	}
	for _, opt := range opts {
		opt(&handler)
	}
	return &handler
}

// Sync Is created on sync of resource
func (t *handler) Sync(ctx context.Context, namespace, name string, eventRecorder record.EventRecorder) error {
	radixJob, err := t.radixclient.RadixV1().RadixJobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		// The Job resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			log.Ctx(ctx).Info().Msgf("RadixJob %s/%s in work queue no longer exists", namespace, name)
			return nil
		}

		return err
	}
	ctx = log.Ctx(ctx).With().Str("app_name", radixJob.Spec.AppName).Logger().WithContext(ctx)

	syncJob := radixJob.DeepCopy()
	log.Ctx(ctx).Debug().Msgf("Sync job %s", syncJob.Name)

	job := job.NewJob(t.kubeclient, t.kubeutil, t.radixclient, syncJob, t.config)
	err = job.OnSync(ctx)
	if err != nil {
		// TODO: should we record a Warning event when there is an error, similar to batch handler? Possibly do it in common.Controller?
		// Put back on queue
		return err
	}

	t.hasSynced(true)
	eventRecorder.Event(syncJob, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// CleanupJobHistory Cleanup the pipeline job history
func (t *handler) CleanupJobHistory(ctx context.Context, radixJob *v1.RadixJob) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Minute*5)
	go func() {
		defer cancel()
		if err := t.jobHistory.Cleanup(ctxWithTimeout, radixJob.Spec.AppName, radixJob.Name); err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("Failed to cleanup job history")
		}
	}()
}
