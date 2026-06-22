package cronserver

import (
	"context"
	"testing"

	testUtil "github.com/equinor/radix-operator/job-scheduler/internal/test"
	"github.com/equinor/radix-operator/job-scheduler/models"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testAppName        = "any-app"
	testEnvName        = "any-env"
	testJobComponent   = "any-job"
	testDeploymentName = "any-deployment"
)

var testNamespace = operatorutils.GetEnvironmentNamespace(testAppName, testEnvName)

// newTestServer builds a cron Server backed by a fake radix client seeded with the given batches.
func newTestServer(t *testing.T, jobComponent *radixv1.RadixDeployJobComponent, batches ...*radixv1.RadixBatch) (*Server, radixclient.Interface) {
	radixClient, _, kubeUtil := testUtil.SetupTest(t, testAppName, testEnvName, testJobComponent, testDeploymentName, 10)
	for _, b := range batches {
		_, err := radixClient.RadixV1().RadixBatches(b.Namespace).Create(context.Background(), b, metav1.CreateOptions{})
		require.NoError(t, err)
	}
	return New(kubeUtil, models.NewEnv(), jobComponent, nil), radixClient
}

// createCronBatch builds a cron RadixBatch for the test job component with the given condition and jobs.
func createCronBatch(name string, condition radixv1.RadixBatchConditionType, jobNames ...string) *radixv1.RadixBatch {
	b := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Labels: radixLabels.Merge(
				radixLabels.ForApplicationName(testAppName),
				radixLabels.ForComponentName(testJobComponent),
				radixLabels.ForBatchCron(true),
			),
		},
		Status: radixv1.RadixBatchStatus{
			Condition: radixv1.RadixBatchCondition{Type: condition},
		},
	}
	for _, jobName := range jobNames {
		b.Spec.Jobs = append(b.Spec.Jobs, radixv1.RadixBatchJob{Name: jobName})
	}
	return b
}

// batchHasStoppedJob reports whether any job in the named batch has been marked Stop=true.
func batchHasStoppedJob(t *testing.T, radixClient radixclient.Interface, batchName string) bool {
	b, err := radixClient.RadixV1().RadixBatches(testNamespace).Get(context.Background(), batchName, metav1.GetOptions{})
	require.NoError(t, err)
	for _, job := range b.Spec.Jobs {
		if job.Stop != nil && *job.Stop {
			return true
		}
	}
	return false
}

func Test_shouldRun(t *testing.T) {
	tests := []struct {
		name        string
		concurrency string
		batches     []*radixv1.RadixBatch
		wantOK      bool
		wantStopped bool
	}{
		{
			name:        "no batches allows run",
			concurrency: concurrencyAllow,
			batches:     nil,
			wantOK:      true,
		},
		{
			name:        "non-active batch is ignored and allows run",
			concurrency: concurrencyForbid,
			batches:     []*radixv1.RadixBatch{createCronBatch("batch1", radixv1.BatchConditionTypeCompleted, "job1")},
			wantOK:      true,
		},
		{
			name:        "active batch with Allow allows run and leaves batch running",
			concurrency: concurrencyAllow,
			batches:     []*radixv1.RadixBatch{createCronBatch("batch1", radixv1.BatchConditionTypeActive, "job1")},
			wantOK:      true,
			wantStopped: false,
		},
		{
			name:        "active batch with Forbid skips run",
			concurrency: concurrencyForbid,
			batches:     []*radixv1.RadixBatch{createCronBatch("batch1", radixv1.BatchConditionTypeActive, "job1")},
			wantOK:      false,
			wantStopped: false,
		},
		{
			name:        "active batch with Replace stops batch and allows run",
			concurrency: concurrencyReplace,
			batches:     []*radixv1.RadixBatch{createCronBatch("batch1", radixv1.BatchConditionTypeActive, "job1")},
			wantOK:      true,
			wantStopped: true,
		},
		{
			name:        "active batch with invalid concurrency skips run",
			concurrency: "NotAValidMode",
			batches:     []*radixv1.RadixBatch{createCronBatch("batch1", radixv1.BatchConditionTypeActive, "job1")},
			wantOK:      false,
			wantStopped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobComponent := &radixv1.RadixDeployJobComponent{
				Name: testJobComponent,
				Cron: &radixv1.CronSchedule{Concurrency: tt.concurrency},
			}
			s, radixClient := newTestServer(t, jobComponent, tt.batches...)

			ok, err := s.shouldRun(context.Background())
			require.NoError(t, err)
			assert.Equal(t, tt.wantOK, ok)

			if len(tt.batches) > 0 {
				assert.Equal(t, tt.wantStopped, batchHasStoppedJob(t, radixClient, tt.batches[0].Name))
			}
		})
	}
}

func Test_Start_timezone(t *testing.T) {
	tests := []struct {
		name     string
		timeZone string
		wantErr  bool
	}{
		{name: "empty timezone defaults to UTC", timeZone: "", wantErr: false},
		{name: "whitespace timezone defaults to UTC", timeZone: "   ", wantErr: false},
		{name: "valid named timezone", timeZone: "Europe/Oslo", wantErr: false},
		{name: "invalid timezone returns error", timeZone: "Not/AZone", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobComponent := &radixv1.RadixDeployJobComponent{
				Name: testJobComponent,
				Cron: &radixv1.CronSchedule{
					TimeZone:  tt.timeZone,
					Schedules: []string{"0 0 1 1 *"}, // once a year, will not fire during the test
				},
			}
			s, _ := newTestServer(t, jobComponent)

			// Cancel up front so Start returns after wiring the scheduler instead of blocking on ctx.Done.
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err := s.Start(ctx)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_Start_invalidScheduleReturnsError(t *testing.T) {
	jobComponent := &radixv1.RadixDeployJobComponent{
		Name: testJobComponent,
		Cron: &radixv1.CronSchedule{
			Schedules: []string{"not-a-valid-cron-expression"},
		},
	}
	s, _ := newTestServer(t, jobComponent)

	err := s.Start(context.Background())
	assert.Error(t, err)
}
