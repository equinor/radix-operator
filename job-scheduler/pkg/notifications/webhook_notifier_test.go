package notifications

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	modelsv1 "github.com/equinor/radix-operator/job-scheduler/models/v1"
	"github.com/equinor/radix-operator/job-scheduler/models/v1/events"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewWebhookNotifier(t *testing.T) {
	tests := []struct {
		name            string
		jobComponent    *radixv1.RadixDeployJobComponent
		expectedEnabled bool
		expectedWebhook string
	}{
		{
			name:            "No notification",
			jobComponent:    &radixv1.RadixDeployJobComponent{},
			expectedEnabled: false,
			expectedWebhook: "",
		},
		{
			name:            "Empty notification",
			jobComponent:    &radixv1.RadixDeployJobComponent{Notifications: &radixv1.Notifications{}},
			expectedEnabled: false,
			expectedWebhook: "",
		},
		{
			name:            "Empty webhook in the notification",
			jobComponent:    &radixv1.RadixDeployJobComponent{Notifications: &radixv1.Notifications{Webhook: pointers.Ptr("")}},
			expectedEnabled: false,
			expectedWebhook: "",
		},
		{
			name:            "Set webhook in the notification",
			jobComponent:    &radixv1.RadixDeployJobComponent{Notifications: &radixv1.Notifications{Webhook: pointers.Ptr("http://job1:8080")}},
			expectedEnabled: true,
			expectedWebhook: "http://job1:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNotifier := NewWebhookNotifier(tt.jobComponent)
			notifier := gotNotifier.(*webhookNotifier)
			assert.Equal(t, tt.expectedEnabled, notifier.Enabled())
			assert.Equal(t, tt.expectedWebhook, notifier.webhookURL)
		})
	}
}

type testTransport struct {
	requestReceived func(request *http.Request) (*http.Response, error)
}

func (t *testTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	_, err := t.requestReceived(request)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
	}, nil
}

func Test_webhookNotifier_Notify(t *testing.T) {
	type fields struct {
		jobComponentName        string
		webhookURL              string
		expectedRequest         bool
		expectedError           bool
		expectedBatchNameInJobs string
	}
	type args struct {
		radixBatch         *radixv1.RadixBatch
		updatedJobStatuses []radixv1.RadixBatchJobStatus
		event              events.Event
	}
	activeTime := metav1.NewTime(time.Date(2020, 10, 30, 1, 1, 1, 1, &time.Location{}))
	completedTime := metav1.NewTime(activeTime.Add(30 * time.Minute))
	startJobTime1 := metav1.NewTime(activeTime.Add(1 * time.Minute))
	startJobTime3 := metav1.NewTime(activeTime.Add(2 * time.Minute))
	endJobTime1 := metav1.NewTime(activeTime.Add(10 * time.Minute))
	endJobTime3 := metav1.NewTime(activeTime.Add(20 * time.Minute))
	jobComponentName := "anyjobcomponent"

	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{name: "No request for notifier with empty webhook",
			fields: fields{jobComponentName: jobComponentName, webhookURL: "", expectedRequest: false, expectedError: false},
			args: args{
				event: events.Update,
				radixBatch: &radixv1.RadixBatch{
					ObjectMeta: metav1.ObjectMeta{Name: "batch1", Labels: labels.ForBatchType(kube.RadixBatchTypeBatch)},
					Status:     radixv1.RadixBatchStatus{Condition: radixv1.RadixBatchCondition{Type: radixv1.BatchConditionTypeWaiting}}},
				updatedJobStatuses: []radixv1.RadixBatchJobStatus{{Name: "job1"}}},
		},
		{name: "No webhook request for other job component",
			fields: fields{jobComponentName: jobComponentName, webhookURL: "http://job1:8080", expectedRequest: false, expectedError: false, expectedBatchNameInJobs: "batch1"},
			args: args{
				event: events.Update,
				radixBatch: &radixv1.RadixBatch{
					ObjectMeta: metav1.ObjectMeta{Name: "batch1", Labels: labels.ForBatchType(kube.RadixBatchTypeBatch)},
					Spec:       radixv1.RadixBatchSpec{RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{Job: "otherjobcomponent"}},
					Status: radixv1.RadixBatchStatus{
						Condition: radixv1.RadixBatchCondition{
							Type:    radixv1.BatchConditionTypeWaiting,
							Reason:  "some reason",
							Message: "some message",
						},
					},
				},
				updatedJobStatuses: []radixv1.RadixBatchJobStatus{}},
		},
		{name: "No webhook request when missing RadixDeploymentJobRef",
			fields: fields{jobComponentName: jobComponentName, webhookURL: "http://job1:8080", expectedRequest: false, expectedError: false, expectedBatchNameInJobs: "batch1"},
			args: args{
				event: events.Update,
				radixBatch: &radixv1.RadixBatch{
					ObjectMeta: metav1.ObjectMeta{Name: "batch1", Labels: labels.ForBatchType(kube.RadixBatchTypeBatch)},
					Status: radixv1.RadixBatchStatus{
						Condition: radixv1.RadixBatchCondition{
							Type:    radixv1.BatchConditionTypeWaiting,
							Reason:  "some reason",
							Message: "some message",
						},
					},
				},
				updatedJobStatuses: []radixv1.RadixBatchJobStatus{}},
		},
		{name: "Waiting batch, no jobs",
			fields: fields{jobComponentName: jobComponentName, webhookURL: "http://job1:8080", expectedRequest: true, expectedError: false, expectedBatchNameInJobs: "batch1"},
			args: args{
				event: events.Update,
				radixBatch: &radixv1.RadixBatch{
					ObjectMeta: metav1.ObjectMeta{Name: "batch1", Labels: labels.ForBatchType(kube.RadixBatchTypeBatch)},
					Spec:       radixv1.RadixBatchSpec{RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{Job: jobComponentName}},
					Status: radixv1.RadixBatchStatus{
						Condition: radixv1.RadixBatchCondition{
							Type:    radixv1.BatchConditionTypeWaiting,
							Reason:  "some reason",
							Message: "some message",
						},
					},
				},
				updatedJobStatuses: []radixv1.RadixBatchJobStatus{}},
		},
		{name: "Active batch",
			fields: fields{jobComponentName: jobComponentName, webhookURL: "http://job1:8080", expectedRequest: true, expectedError: false, expectedBatchNameInJobs: "batch1"},
			args: args{
				event: events.Update,
				radixBatch: &radixv1.RadixBatch{
					ObjectMeta: metav1.ObjectMeta{Name: "batch1", Labels: labels.ForBatchType(kube.RadixBatchTypeBatch)},
					Spec:       radixv1.RadixBatchSpec{RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{Job: jobComponentName}},
					Status: radixv1.RadixBatchStatus{
						Condition: radixv1.RadixBatchCondition{
							Type:           radixv1.BatchConditionTypeActive,
							Reason:         "some reason",
							Message:        "some message",
							CompletionTime: &completedTime,
							ActiveTime:     &activeTime,
						},
					},
				},
				updatedJobStatuses: []radixv1.RadixBatchJobStatus{}},
		},
		{name: "Completed Batch with multiple jobs",
			fields: fields{jobComponentName: jobComponentName, webhookURL: "http://job1:8080", expectedRequest: true, expectedError: false, expectedBatchNameInJobs: "batch1"},
			args: args{
				event: events.Update,
				radixBatch: &radixv1.RadixBatch{
					ObjectMeta: metav1.ObjectMeta{Name: "batch1", Labels: labels.ForBatchType(kube.RadixBatchTypeBatch)},
					Spec: radixv1.RadixBatchSpec{
						RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{Job: jobComponentName},
						Jobs:                  []radixv1.RadixBatchJob{{Name: "job1"}, {Name: "job2"}, {Name: "job3"}},
					},
					Status: radixv1.RadixBatchStatus{
						Condition: radixv1.RadixBatchCondition{
							Type:           radixv1.BatchConditionTypeCompleted,
							Reason:         "some reason",
							Message:        "some message",
							CompletionTime: &completedTime,
							ActiveTime:     &activeTime,
						},
					},
				},
				updatedJobStatuses: []radixv1.RadixBatchJobStatus{
					{
						Name: "job1",
					},
					{
						Name:      "job2",
						Phase:     "some job phase",
						StartTime: &startJobTime1,
					},
					{
						Name:      "job3",
						Phase:     "some job phase 753",
						Reason:    "some reason 123",
						Message:   "some message 456",
						StartTime: &startJobTime3,
						EndTime:   &endJobTime3,
					},
				}},
		},
		{name: "Completed single job batch",
			fields: fields{jobComponentName: jobComponentName, webhookURL: "http://job1:8080", expectedRequest: true, expectedError: false, expectedBatchNameInJobs: ""},
			args: args{
				event: events.Update,
				radixBatch: &radixv1.RadixBatch{
					ObjectMeta: metav1.ObjectMeta{Name: "batch1", Labels: labels.ForBatchType(kube.RadixBatchTypeJob)},
					Spec: radixv1.RadixBatchSpec{
						RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{Job: jobComponentName},
						Jobs:                  []radixv1.RadixBatchJob{{Name: "job1"}},
					},
					Status: radixv1.RadixBatchStatus{
						Condition: radixv1.RadixBatchCondition{
							Type:           radixv1.BatchConditionTypeCompleted,
							Reason:         "some reason",
							Message:        "some message",
							CompletionTime: &completedTime,
							ActiveTime:     &activeTime,
						},
					},
				},
				updatedJobStatuses: []radixv1.RadixBatchJobStatus{
					{
						Name:      "job1",
						Phase:     "some job phase 753",
						Reason:    "some reason 123",
						Message:   "some message 456",
						StartTime: &startJobTime1,
						EndTime:   &endJobTime1,
					},
				}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobComponent := &radixv1.RadixDeployJobComponent{
				Name:          tt.fields.jobComponentName,
				Notifications: &radixv1.Notifications{Webhook: pointers.Ptr(tt.fields.webhookURL)},
			}
			notifier := NewWebhookNotifier(jobComponent)
			var receivedRequest *http.Request
			http.DefaultClient = &http.Client{
				Transport: &testTransport{
					requestReceived: func(request *http.Request) (*http.Response, error) {
						receivedRequest = request
						return &http.Response{
							Status:     "200 OK",
							StatusCode: 200,
						}, nil
					},
				},
			}

			notificationErr := notifier.Notify(tt.args.event, tt.args.radixBatch, tt.args.updatedJobStatuses)

			if tt.fields.expectedRequest && receivedRequest == nil {
				assert.Fail(t, "missing an expected http request")
				return
			} else if !tt.fields.expectedRequest && receivedRequest != nil {
				assert.Fail(t, "received a not expected http request")
				return
			}
			if tt.fields.expectedError && notificationErr == nil {
				assert.Fail(t, "missing an expected notification error")
				return
			} else if !tt.fields.expectedError && notificationErr != nil {
				assert.Fail(t, fmt.Sprintf("received a not expected notification error %v", notificationErr))
				return
			}
			if receivedRequest != nil {
				assert.Equal(t, tt.fields.webhookURL, fmt.Sprintf("%s://%s", receivedRequest.URL.Scheme, receivedRequest.Host))
				var batchStatus modelsv1.BatchStatus
				if body, _ := io.ReadAll(receivedRequest.Body); len(body) > 0 {
					if err := json.Unmarshal(body, &batchStatus); err != nil {
						assert.Fail(t, fmt.Sprintf("failed to deserialize the request body: %v", err))
						return
					}
				}
				assert.Equal(t, tt.args.radixBatch.Name, batchStatus.Name, "Not matching batch name")
				assertTimesEqual(t, tt.args.radixBatch.Status.Condition.ActiveTime, batchStatus.Started, "batchStatus.Started")
				assertTimesEqual(t, tt.args.radixBatch.Status.Condition.CompletionTime, batchStatus.Ended, "batchStatus.Ended")
				if len(tt.args.updatedJobStatuses) != len(batchStatus.JobStatuses) {
					assert.Fail(t, fmt.Sprintf("Not matching amount of updatedJobStatuses %d and JobStatuses %d", len(tt.args.updatedJobStatuses), len(batchStatus.JobStatuses)))
					return
				}
				for index, updateJobsStatus := range tt.args.updatedJobStatuses {
					jobStatus := batchStatus.JobStatuses[index]
					assert.Equal(t, tt.fields.expectedBatchNameInJobs, jobStatus.BatchName)
					assert.Equal(t, fmt.Sprintf("%s-%s", batchStatus.Name, updateJobsStatus.Name), jobStatus.Name)
					assertTimesEqual(t, updateJobsStatus.StartTime, jobStatus.Started, "job.Started")
					assertTimesEqual(t, updateJobsStatus.EndTime, jobStatus.Ended, "job.EndTime")
				}
			}
		})
	}
}

func assertTimesEqual(t *testing.T, expectedTime *metav1.Time, resultTime *time.Time, arg string) {
	if expectedTime != nil && resultTime == nil {
		assert.Fail(t, fmt.Sprintf("missing an expected %s", arg))
		return
	} else if expectedTime == nil && resultTime != nil {
		assert.Fail(t, fmt.Sprintf("got a not expected %s", arg))
		return
	}
	if expectedTime != nil {
		assert.WithinDuration(t, expectedTime.Time, *resultTime, 1, arg)
	}
}
