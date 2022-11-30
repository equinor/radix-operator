package job

import (
	"sort"

	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

func sortJobsByCreatedAsc(rjs []v1.RadixJob) []v1.RadixJob {
	sort.Slice(rjs, func(i, j int) bool {
		return isCreatedAfter(&rjs[j], &rjs[i])
	})
	return rjs
}

func sortJobsByCreatedDesc(rjs []v1.RadixJob) []v1.RadixJob {
	sort.Slice(rjs, func(i, j int) bool {
		return isCreatedBefore(&rjs[j], &rjs[i])
	})
	return rjs
}

func isCreatedAfter(rj1 *v1.RadixJob, rj2 *v1.RadixJob) bool {
	rj1Created := rj1.Status.Created
	rj2Created := rj2.Status.Created

	return rj1Created.After(rj2Created.Time)
}

func isCreatedBefore(rj1 *v1.RadixJob, rj2 *v1.RadixJob) bool {
	rj1Created := rj1.Status.Created
	rj2Created := rj2.Status.Created

	return rj1Created.Before(rj2Created)
}
