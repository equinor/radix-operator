package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	modelsv1 "github.com/equinor/radix-operator/job-scheduler/models/v1"
	"github.com/equinor/radix-operator/job-scheduler/models/v1/events"
	"github.com/equinor/radix-operator/job-scheduler/pkg/internal"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
)

type webhookNotifier struct {
	webhookURL              string
	jobComponentName        string
	radixDeployJobComponent *radixv1.RadixDeployJobComponent
}

func NewWebhookNotifier(radixDeployJobComponent *radixv1.RadixDeployJobComponent) Notifier {
	notifier := webhookNotifier{
		jobComponentName:        radixDeployJobComponent.Name,
		radixDeployJobComponent: radixDeployJobComponent,
	}
	if radixDeployJobComponent.Notifications != nil && webhookIsNotEmpty(radixDeployJobComponent.Notifications.Webhook) {
		notifier.webhookURL = *radixDeployJobComponent.Notifications.Webhook
	}
	return &notifier
}

func (notifier *webhookNotifier) Enabled() bool {
	return len(notifier.webhookURL) > 0
}

func (notifier *webhookNotifier) String() string {
	if notifier.Enabled() {
		return fmt.Sprintf("Webhook notifier is enabled. Webhook: %s", notifier.webhookURL)
	}
	return "Webhook notifier is disabled"
}

func (notifier *webhookNotifier) Notify(event events.Event, radixBatch *radixv1.RadixBatch, updatedJobStatuses []radixv1.RadixBatchJobStatus) error {
	if !notifier.Enabled() || len(notifier.webhookURL) == 0 || radixBatch.Spec.RadixDeploymentJobRef.Job != notifier.jobComponentName {
		return nil
	}
	// BatchStatus status and only changed job statuses
	batchStatus := getRadixBatchEventFromRadixBatch(event, radixBatch, updatedJobStatuses, notifier.radixDeployJobComponent)
	statusesJson, err := json.Marshal(batchStatus)
	if err != nil {
		return fmt.Errorf("failed serialize updated JobStatuses %v", err)
	}
	log.Trace().Msg(string(statusesJson))
	buf := bytes.NewReader(statusesJson)
	if _, err = http.Post(notifier.webhookURL, "application/json", buf); err != nil {
		return fmt.Errorf("failed to notify on BatchStatus object create or change %s: %v", radixBatch.GetName(), err)
	}
	return nil
}

func webhookIsNotEmpty(webhook *string) bool {
	return webhook != nil && len(*webhook) > 0
}

func getRadixBatchEventFromRadixBatch(event events.Event, radixBatch *radixv1.RadixBatch, radixBatchJobStatuses []radixv1.RadixBatchJobStatus, radixDeployJobComponent *radixv1.RadixDeployJobComponent) events.BatchEvent {
	batchStatus, jobStatuses := getBatchAndJobStatuses(radixBatch, radixDeployJobComponent, radixBatchJobStatuses)
	return events.BatchEvent{
		Event: event,
		BatchStatus: modelsv1.BatchStatus{
			JobStatus:   batchStatus,
			JobStatuses: jobStatuses,
			BatchType:   radixBatch.Labels[kube.RadixBatchTypeLabel],
		},
	}
}

func getBatchAndJobStatuses(radixBatch *radixv1.RadixBatch, radixDeployJobComponent *radixv1.RadixDeployJobComponent, radixBatchJobStatuses []radixv1.RadixBatchJobStatus) (modelsv1.JobStatus, []modelsv1.JobStatus) {
	var startedTime, endedTime *time.Time
	if radixBatch.Status.Condition.ActiveTime != nil {
		startedTime = &radixBatch.Status.Condition.ActiveTime.Time
	}
	if radixBatch.Status.Condition.CompletionTime != nil {
		endedTime = &radixBatch.Status.Condition.CompletionTime.Time
	}

	batchStatus := modelsv1.JobStatus{
		Name:    radixBatch.GetName(),
		BatchId: internal.GetBatchId(radixBatch),
		Created: pointers.Ptr(radixBatch.GetCreationTimestamp().Time),
		Started: startedTime,
		Ended:   endedTime,
		Status:  internal.GetRadixBatchStatus(radixBatch, radixDeployJobComponent),
		Message: radixBatch.Status.Condition.Message,
		Updated: pointers.Ptr(time.Now()),
	}
	jobStatuses := getRadixBatchJobStatusesFromRadixBatch(radixBatch, radixBatchJobStatuses)
	return batchStatus, jobStatuses
}

func getRadixBatchJobsMap(radixBatchJobs []radixv1.RadixBatchJob) map[string]radixv1.RadixBatchJob {
	jobMap := make(map[string]radixv1.RadixBatchJob, len(radixBatchJobs))
	for _, radixBatchJob := range radixBatchJobs {
		jobMap[radixBatchJob.Name] = radixBatchJob
	}
	return jobMap
}

func getRadixBatchJobStatusesFromRadixBatch(radixBatch *radixv1.RadixBatch, radixBatchJobStatuses []radixv1.RadixBatchJobStatus) []modelsv1.JobStatus {
	batchName := internal.GetBatchName(radixBatch)
	radixBatchJobsMap := getRadixBatchJobsMap(radixBatch.Spec.Jobs)
	jobStatuses := make([]modelsv1.JobStatus, 0, len(radixBatchJobStatuses))
	for _, radixBatchJobStatus := range radixBatchJobStatuses {
		var started, ended, created *time.Time
		if radixBatchJobStatus.CreationTime != nil {
			created = &radixBatchJobStatus.CreationTime.Time
		}
		if radixBatchJobStatus.StartTime != nil {
			started = &radixBatchJobStatus.StartTime.Time
		}

		if radixBatchJobStatus.EndTime != nil {
			ended = &radixBatchJobStatus.EndTime.Time
		}

		radixBatchJob, ok := radixBatchJobsMap[radixBatchJobStatus.Name]
		if !ok {
			continue
		}
		stopJob := radixBatchJob.Stop != nil && *radixBatchJob.Stop
		jobName := fmt.Sprintf("%s-%s", radixBatch.Name, radixBatchJobStatus.Name) // composed name in models are always consist of a batchName and original jobName
		jobStatus := modelsv1.JobStatus{
			BatchName:   batchName,
			Name:        jobName,
			JobId:       radixBatchJob.JobId,
			Created:     created,
			Started:     started,
			Ended:       ended,
			Status:      string(internal.GetScheduledJobStatus(radixBatchJobStatus, stopJob)),
			Failed:      radixBatchJobStatus.Failed,
			Restart:     radixBatchJobStatus.Restart,
			Message:     radixBatchJobStatus.Message,
			Updated:     pointers.Ptr(time.Now()),
			PodStatuses: internal.GetPodStatusByRadixBatchJobPodStatus(radixBatch, radixBatchJobStatus.RadixBatchJobPodStatuses),
		}
		jobStatuses = append(jobStatuses, jobStatus)
	}
	return jobStatuses
}
