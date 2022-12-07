package job

import (
	"sort"

	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

func sortRadixJobsByCreatedAsc(radixJobs []v1.RadixJob) []v1.RadixJob {
	sort.Slice(radixJobs, func(i, j int) bool {
		return isCreatedAfter(&radixJobs[j], &radixJobs[i])
	})
	return radixJobs
}

func sortRadixJobsByCreatedDesc(radixJobs []v1.RadixJob) []v1.RadixJob {
	sort.Slice(radixJobs, func(i, j int) bool {
		return isCreatedBefore(&radixJobs[j], &radixJobs[i])
	})
	return radixJobs
}

func isCreatedAfter(rj1 *v1.RadixJob, rj2 *v1.RadixJob) bool {
	rj1Created := rj1.Status.Created
	if rj1Created == nil {
		return true
	}
	rj2Created := rj2.Status.Created
	if rj2Created == nil {
		return false
	}
	return rj1Created.After(rj2Created.Time)
}

func isCreatedBefore(rj1 *v1.RadixJob, rj2 *v1.RadixJob) bool {
	return !isCreatedAfter(rj1, rj2)
}
