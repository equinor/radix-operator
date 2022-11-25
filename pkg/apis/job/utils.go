package job

import (
	"sort"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

func sortJobsByActiveFromAsc(rjs []v1.RadixJob) []v1.RadixJob {
	sort.Slice(rjs, func(i, j int) bool {
		return isRJ1ActiveAfterRJ2(&rjs[j], &rjs[i])
	})
	return rjs
}

func sortJobsByActiveFromDesc(rjs []v1.RadixJob) []v1.RadixJob {
	sort.Slice(rjs, func(i, j int) bool {
		return isRJ1ActiveBeforeRJ2(&rjs[j], &rjs[i])
	})
	return rjs
}

func isRJ1ActiveAfterRJ2(rj1 *v1.RadixJob, rj2 *v1.RadixJob) bool {
	rj1ActiveFrom := rj1.CreationTimestamp
	rj2ActiveFrom := rj2.CreationTimestamp

	return rj2ActiveFrom.Before(&rj1ActiveFrom)
}

func isRJ1ActiveBeforeRJ2(rj1 *v1.RadixJob, rj2 *v1.RadixJob) bool {
	rj1ActiveFrom := rj1.CreationTimestamp
	rj2ActiveFrom := rj2.CreationTimestamp

	return rj1ActiveFrom.Before(&rj2ActiveFrom)
}
