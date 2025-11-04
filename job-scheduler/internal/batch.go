package internal

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/job-scheduler/pkg/errors"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var defaultSrc = rand.NewSource(time.Now().UnixNano())

// GetRadixBatches Get Radix batches
func GetRadixBatches(ctx context.Context, radixClient radixclient.Interface, namespace string, labels ...map[string]string) ([]*radixv1.RadixBatch, error) {
	radixBatchList, err := radixClient.
		RadixV1().
		RadixBatches(namespace).
		List(
			ctx,
			metav1.ListOptions{
				LabelSelector: radixLabels.Merge(labels...).String(),
			},
		)

	if err != nil {
		return nil, err
	}

	return slice.PointersOf(radixBatchList.Items).([]*radixv1.RadixBatch), nil
}

// GenerateBatchName Generate batch name
func GenerateBatchName(jobComponentName string) string {
	timestamp := time.Now().Format("20060102150405")
	return fmt.Sprintf("batch-%s-%s-%s", getJobComponentNamePart(jobComponentName), timestamp, strings.ToLower(utils.RandString(8)))
}

func getJobComponentNamePart(jobComponentName string) string {
	componentNamePart := jobComponentName
	if len(componentNamePart) > 12 {
		componentNamePart = componentNamePart[:12]
	}
	return fmt.Sprintf("%s%s", componentNamePart, strings.ToLower(utils.RandString(16-len(componentNamePart))))
}

// CreateJobName create a job name
func CreateJobName() string {
	return strings.ToLower(utils.RandStringSeed(8, defaultSrc))
}

// GetRadixBatch Get Radix batch
func GetRadixBatch(ctx context.Context, radixClient radixclient.Interface, namespace, batchName string) (*radixv1.RadixBatch, error) {
	radixBatch, err := radixClient.RadixV1().RadixBatches(namespace).Get(ctx, batchName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.NewFromError(err)
	}
	return radixBatch, nil
}

// ParseBatchAndJobNameFromScheduledJobName Decompose batch name and jobs name from job-name
func ParseBatchAndJobNameFromScheduledJobName(scheduleJobName string) (batchName, batchJobName string, ok bool) {
	scheduleJobNameParts := strings.Split(scheduleJobName, "-")
	if len(scheduleJobNameParts) < 2 {
		return
	}
	batchName = strings.Join(scheduleJobNameParts[:len(scheduleJobNameParts)-1], "-")
	batchJobName = scheduleJobNameParts[len(scheduleJobNameParts)-1]
	ok = true
	return
}
