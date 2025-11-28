package job

import (
	"context"
	"time"

	"github.com/equinor/radix-operator/operator/common"
	apiconfig "github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

// Handler Common handler interface
type Handler interface {
	common.Handler
	// CleanupJobHistory Cleanup the pipeline job history for the Radix application
	CleanupJobHistory(ctx context.Context, appName string)
}

type handler struct {
	kubeclient  kubernetes.Interface
	radixclient radixclient.Interface
	kubeutil    *kube.Kube
	events      common.SyncEventRecorder
	config      *apiconfig.Config
	jobHistory  job.History
}

type handlerOpts func(*handler)

// NewHandler Constructor
func NewHandler(kubeclient kubernetes.Interface,
	kubeUtil *kube.Kube,
	radixClient radixclient.Interface,
	eventRecorder record.EventRecorder,
	config *apiconfig.Config,
	opts ...handlerOpts) Handler {

	handler := &handler{
		kubeclient:  kubeclient,
		radixclient: radixClient,
		kubeutil:    kubeUtil,
		events:      common.NewSyncEventRecorder(eventRecorder),
		config:      config,
		jobHistory:  job.NewHistory(radixClient, kubeUtil, config.PipelineJobConfig.PipelineJobsHistoryLimit, config.PipelineJobConfig.PipelineJobsHistoryPeriodLimit),
	}
	for _, opt := range opts {
		opt(handler)
	}
	return handler
}

// Sync Is created on sync of resource
func (t *handler) Sync(ctx context.Context, namespace, jobName string) error {
	radixJob, err := t.radixclient.RadixV1().RadixJobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		// The Job resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			log.Ctx(ctx).Info().Msgf("RadixJob %s/%s in work queue no longer exists", namespace, jobName)
			return nil
		}

		return err
	}
	logger := log.Ctx(ctx).With().Str("app_name", radixJob.Spec.AppName).Logger()

	radixRegistration, err := t.radixclient.RadixV1().RadixRegistrations().Get(ctx, radixJob.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		// The Registration resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			logger.Debug().Msgf("RadixRegistration %s no longer exists", radixJob.Spec.AppName)
			return nil
		}

		return err
	}

	syncJob := radixJob.DeepCopy()
	logger.Debug().Msgf("Sync job %s", syncJob.Name)
	ctx = logger.WithContext(ctx)

	syncer := job.NewJob(t.kubeclient, t.kubeutil, t.radixclient, radixRegistration, syncJob, t.config)
	if err = syncer.OnSync(ctx); err != nil {
		t.events.RecordSyncErrorEvent(syncJob, err)
		return err
	}

	t.events.RecordSyncSuccessEvent(syncJob)
	return nil
}

// CleanupJobHistory Cleanup the pipeline job history
func (t *handler) CleanupJobHistory(ctx context.Context, appName string) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Minute*5)
	go func() {
		defer cancel()
		if err := t.jobHistory.Cleanup(ctxWithTimeout, appName); err != nil {
			log.Ctx(ctx).Error().Err(err).Msgf("Failed to cleanup job historyfor the Radix application %s", appName)
		}
	}()
}
