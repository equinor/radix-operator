package watcher

import (
	"context"
	"testing"
	"time"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/job-scheduler/models/v1/events"
	"github.com/equinor/radix-operator/job-scheduler/pkg/batch"
	"github.com/equinor/radix-operator/job-scheduler/pkg/notifications"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclientfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_RadixBatchWatcher(t *testing.T) {
	activeTime := metav1.NewTime(time.Date(2020, 10, 30, 1, 1, 1, 1, &time.Location{}))
	startJobTime1 := metav1.NewTime(activeTime.Add(1 * time.Minute))
	startJobTime2 := metav1.NewTime(activeTime.Add(2 * time.Minute))
	startJobTime3 := metav1.NewTime(activeTime.Add(3 * time.Minute))
	endJobTime2 := metav1.NewTime(activeTime.Add(10 * time.Minute))
	endJobTime3 := metav1.NewTime(activeTime.Add(12 * time.Minute))
	type fields struct {
		newRadixBatch    *radixv1.RadixBatch
		updateRadixBatch func(*radixv1.RadixBatch) *radixv1.RadixBatch
		event            events.Event
	}
	type args struct {
		getNotifier func(*gomock.Controller) notifications.Notifier
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "No batch, no notifications",
			fields: fields{
				newRadixBatch: nil,
			},
			args: args{
				getNotifier: func(ctrl *gomock.Controller) notifications.Notifier {
					return notifications.NewMockNotifier(ctrl)
				},
			},
		},
		{
			name: "Created batch, sends notification about batch without status",
			fields: fields{
				newRadixBatch: &radixv1.RadixBatch{
					ObjectMeta: metav1.ObjectMeta{Name: "batch1", Labels: labels.ForBatchType(kube.RadixBatchTypeBatch)},
					Spec: radixv1.RadixBatchSpec{
						Jobs: []radixv1.RadixBatchJob{{Name: "job1"}},
					},
				},
				updateRadixBatch: nil,
				event:            events.Create,
			},
			args: args{
				getNotifier: func(ctrl *gomock.Controller) notifications.Notifier {
					notifier := notifications.NewMockNotifier(ctrl)
					rbMatcher := newRadixBatchMatcher(func(radixBatch *radixv1.RadixBatch) bool {
						return radixBatch.Name == "batch1" && radixBatch.Status.Condition == radixv1.RadixBatchCondition{}
					})
					notifier.EXPECT().Notify(events.Create, rbMatcher,
						[]radixv1.RadixBatchJobStatus{}).Times(1)
					return notifier
				},
			},
		},
		{
			name: "Updated only batch status, sends notification about batch status only",
			fields: fields{
				newRadixBatch: &radixv1.RadixBatch{
					ObjectMeta: metav1.ObjectMeta{Name: "batch1", Labels: labels.ForBatchType(kube.RadixBatchTypeBatch)},
					Spec: radixv1.RadixBatchSpec{
						Jobs: []radixv1.RadixBatchJob{{Name: "job1"}},
					},
				},
				updateRadixBatch: func(radixBatch *radixv1.RadixBatch) *radixv1.RadixBatch {
					radixBatch.Status.Condition.Type = radixv1.BatchConditionTypeWaiting
					return radixBatch
				},
				event: events.Update,
			},
			args: args{
				getNotifier: func(ctrl *gomock.Controller) notifications.Notifier {
					notifier := notifications.NewMockNotifier(ctrl)
					rbMatcher := newRadixBatchMatcher(func(radixBatch *radixv1.RadixBatch) bool {
						return radixBatch.Name == "batch1" &&
							radixBatch.Status.Condition.Type == radixv1.BatchConditionTypeWaiting
					})
					notifier.EXPECT().Notify(events.Update, rbMatcher,
						[]radixv1.RadixBatchJobStatus{}).Times(1)
					return notifier
				},
			},
		},
		{
			name: "Updated batch job statuses, sends notification only about changed batch job status",
			fields: fields{
				newRadixBatch: &radixv1.RadixBatch{
					ObjectMeta: metav1.ObjectMeta{Name: "batch1", Labels: labels.ForBatchType(kube.RadixBatchTypeBatch)},
					Spec: radixv1.RadixBatchSpec{
						Jobs: []radixv1.RadixBatchJob{{Name: "job1"}},
					},
					Status: radixv1.RadixBatchStatus{
						Condition: radixv1.RadixBatchCondition{
							ActiveTime: &activeTime,
						},
						JobStatuses: []radixv1.RadixBatchJobStatus{
							{
								Name:      "job1",
								Reason:    "job1 reason",
								StartTime: &startJobTime1,
							},
							{
								Name: "job2",
							},
							{
								Name:      "job3",
								Reason:    "job3 reason",
								Message:   "job3 message",
								StartTime: &startJobTime3,
								EndTime:   &endJobTime3,
							},
						},
					},
				},
				updateRadixBatch: func(radixBatch *radixv1.RadixBatch) *radixv1.RadixBatch {
					radixBatch.Status.JobStatuses[1].StartTime = &startJobTime2
					radixBatch.Status.JobStatuses[1].Reason = "job2 reason"
					radixBatch.Status.JobStatuses[1].Message = "job2 message"
					radixBatch.Status.JobStatuses[1].Phase = "job2 phase"
					radixBatch.Status.JobStatuses[1].EndTime = &endJobTime2
					return radixBatch
				},
				event: events.Update,
			},
			args: args{
				getNotifier: func(ctrl *gomock.Controller) notifications.Notifier {
					notifier := notifications.NewMockNotifier(ctrl)
					rbMatcher := newRadixBatchMatcher(func(radixBatch *radixv1.RadixBatch) bool {
						return radixBatch.Name == "batch1" && *radixBatch.Status.Condition.ActiveTime == activeTime
					})
					jobStatusesMatcher := newRadixBatchJobStatusMatcher(func(jobStatuses []radixv1.RadixBatchJobStatus) bool {
						return len(jobStatuses) == 1 &&
							jobStatuses[0].Name == "job2" &&
							jobStatuses[0].Reason == "job2 reason" &&
							jobStatuses[0].Message == "job2 message" &&
							jobStatuses[0].Phase == "job2 phase" &&
							*jobStatuses[0].StartTime == startJobTime2 &&
							*jobStatuses[0].EndTime == endJobTime2
					})
					notifier.EXPECT().Notify(events.Update, rbMatcher,
						jobStatusesMatcher).Times(1)
					return notifier
				},
			},
		},
		{
			name: "Deleted batch status, sends notification about batch status",
			fields: fields{
				newRadixBatch: &radixv1.RadixBatch{
					ObjectMeta: metav1.ObjectMeta{Name: "batch1", Labels: labels.ForBatchType(kube.RadixBatchTypeBatch)},
					Spec: radixv1.RadixBatchSpec{
						Jobs: []radixv1.RadixBatchJob{{Name: "job1"}},
					},
				},
				event: events.Delete,
			},
			args: args{
				getNotifier: func(ctrl *gomock.Controller) notifications.Notifier {
					notifier := notifications.NewMockNotifier(ctrl)
					rbMatcher := newRadixBatchMatcher(func(radixBatch *radixv1.RadixBatch) bool {
						return radixBatch.Name == "batch1"
					})
					notifier.EXPECT().Notify(events.Delete, rbMatcher,
						[]radixv1.RadixBatchJobStatus{}).Times(1)
					return notifier
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt // Must capture outer loop variable when t.Run func calls
		t.Run(tt.name, func(t *testing.T) {
			radixClient := radixclientfake.NewSimpleClientset()
			namespace := "app-qa"
			var createdRadixBatch *radixv1.RadixBatch
			var err error
			if tt.fields.newRadixBatch != nil && (tt.fields.event == events.Update || tt.fields.event == events.Delete) {
				// when radix batch exists and during test it will be updated
				createdRadixBatch, err = radixClient.RadixV1().RadixBatches(namespace).Create(context.TODO(), tt.fields.newRadixBatch, metav1.CreateOptions{})
				if err != nil {
					assert.Fail(t, err.Error())
					return
				}
			}

			ctrl := gomock.NewController(t)
			history := batch.NewMockHistory(ctrl)
			batchWatcher, err := NewRadixBatchWatcher(context.Background(), radixClient, namespace, history, tt.args.getNotifier(ctrl))
			defer batchWatcher.Stop()
			if err != nil {
				assert.Fail(t, err.Error())
				return
			}
			assert.False(t, commonUtils.IsNil(batchWatcher))

			if tt.fields.newRadixBatch != nil && tt.fields.event == events.Create {
				history.EXPECT().Cleanup(gomock.Any(), make(map[string]struct{})).Times(1)
				// when radix batch exists and during test it will be updated
				_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.TODO(), tt.fields.newRadixBatch, metav1.CreateOptions{})
				if err != nil {
					assert.Fail(t, err.Error())
					return
				}
			} else if createdRadixBatch != nil {
				if tt.fields.event == events.Update {
					_, err := radixClient.RadixV1().RadixBatches(namespace).Update(context.TODO(), tt.fields.updateRadixBatch(createdRadixBatch), metav1.UpdateOptions{})
					if err != nil {
						assert.Fail(t, err.Error())
						return
					}
				}
				if tt.fields.event == events.Delete {
					err := radixClient.RadixV1().RadixBatches(namespace).Delete(context.TODO(), tt.fields.newRadixBatch.Name, metav1.DeleteOptions{})
					if err != nil {
						assert.Fail(t, err.Error())
						return
					}
				}
			}
			time.Sleep(time.Second * 1) // wait to possible fail due to a missed expected call
			ctrl.Finish()
		})
	}
}

func newRadixBatchMatcher(matches func(*radixv1.RadixBatch) bool) *radixBatchMatcher {
	return &radixBatchMatcher{matches: matches}
}

type radixBatchMatcher struct {
	matches func(*radixv1.RadixBatch) bool
}

func (m *radixBatchMatcher) Matches(x interface{}) bool {
	radixBatch := x.(*radixv1.RadixBatch)
	return m.matches(radixBatch)
}

func (m *radixBatchMatcher) String() string {
	return "radixBatch matcher"
}

func newRadixBatchJobStatusMatcher(matches func([]radixv1.RadixBatchJobStatus) bool) *radixBatchJobStatusesMatcher {
	return &radixBatchJobStatusesMatcher{matches: matches}
}

type radixBatchJobStatusesMatcher struct {
	matches func([]radixv1.RadixBatchJobStatus) bool
}

func (m *radixBatchJobStatusesMatcher) Matches(x interface{}) bool {
	radixBatchJobStatuses := x.([]radixv1.RadixBatchJobStatus)
	return m.matches(radixBatchJobStatuses)
}

func (m *radixBatchJobStatusesMatcher) String() string {
	return "radixBatchJobStatuses matcher"
}
