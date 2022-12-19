package job

import (
	"fmt"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type testSuite struct {
	suite.Suite
	timeNow time.Time
}

type jobPair struct {
	rj1 *v1.RadixJob
	rj2 *v1.RadixJob
}

type singleScenario struct {
	name string
	jobs jobPair
	want bool
}

type listScenario struct {
	name                   string
	jobs                   []v1.RadixJob
	expectedJobNamesInList []string
}

func TestSuite(t *testing.T) {
	testSuite := new(testSuite)
	testSuite.timeNow = time.Now()
	suite.Run(t, testSuite)
}

func (s *testSuite) Test_isCreatedAfter() {
	tests := []singleScenario{
		{
			name: "rj1 is not after rj2",
			jobs: jobPair{
				rj1: createRadixJob("rj1", getPtr(s.timeNow)),
				rj2: createRadixJob("rj2", getPtr(s.timeNow.Add(time.Hour))),
			},
			want: false,
		},
		{
			name: "rj1 is after rj2",
			jobs: jobPair{
				rj1: createRadixJob("rj1", getPtr(s.timeNow.Add(time.Hour))),
				rj2: createRadixJob("rj2", getPtr(s.timeNow)),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equalf(tt.want, isCreatedAfter(tt.jobs.rj1, tt.jobs.rj2), "isCreatedAfter(%s, %s)", tt.jobs.rj1.GetName(), tt.jobs.rj2.GetName())
		})
	}
}

func (s *testSuite) Test_isCreatedBefore() {
	tests := []singleScenario{
		{
			name: "rj1 is before rj2",
			jobs: jobPair{
				rj1: createRadixJob("rj1", getPtr(s.timeNow)),
				rj2: createRadixJob("rj2", getPtr(s.timeNow.Add(time.Hour))),
			},
			want: true,
		},
		{
			name: "rj1 is not before rj2",
			jobs: jobPair{
				rj1: createRadixJob("rj1", getPtr(s.timeNow.Add(time.Hour))),
				rj2: createRadixJob("rj2", getPtr(s.timeNow)),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equalf(tt.want, isCreatedBefore(tt.jobs.rj1, tt.jobs.rj2), "isCreatedBefore(%s, %s)", tt.jobs.rj1.GetName(), tt.jobs.rj2.GetName())
		})
	}
}

func (s *testSuite) Test_sortJobsByCreatedAsc() {
	tests := []listScenario{
		{
			name: "asc order",
			jobs: []v1.RadixJob{
				*createRadixJob("rj1", getPtr(s.timeNow)),
				*createRadixJob("rj2", getPtr(s.timeNow.Add(time.Hour))),
				*createRadixJob("rj3", getPtr(s.timeNow.Add(time.Hour*2))),
			},
			expectedJobNamesInList: []string{"rj1", "rj2", "rj3"},
		},
		{
			name: "desc",
			jobs: []v1.RadixJob{
				*createRadixJob("rj1", getPtr(s.timeNow.Add(time.Hour*2))),
				*createRadixJob("rj2", getPtr(s.timeNow.Add(time.Hour))),
				*createRadixJob("rj3", getPtr(s.timeNow)),
			},
			expectedJobNamesInList: []string{"rj3", "rj2", "rj1"},
		},
		{
			name: "random",
			jobs: []v1.RadixJob{
				*createRadixJob("rj1", getPtr(s.timeNow.Add(time.Hour))),
				*createRadixJob("rj2", getPtr(s.timeNow)),
				*createRadixJob("rj3", getPtr(s.timeNow.Add(time.Hour*2))),
			},
			expectedJobNamesInList: []string{"rj2", "rj1", "rj3"},
		},
		{
			name: "not set created on rj1",
			jobs: []v1.RadixJob{
				*createRadixJob("rj1", nil),
				*createRadixJob("rj2", getPtr(s.timeNow)),
				*createRadixJob("rj3", getPtr(s.timeNow.Add(time.Hour*2))),
			},
			expectedJobNamesInList: []string{"rj2", "rj3", "rj1"},
		},
		{
			name: "not set created on rj2",
			jobs: []v1.RadixJob{
				*createRadixJob("rj1", getPtr(s.timeNow)),
				*createRadixJob("rj2", nil),
				*createRadixJob("rj3", getPtr(s.timeNow.Add(time.Hour*2))),
			},
			expectedJobNamesInList: []string{"rj1", "rj3", "rj2"},
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := sortRadixJobsByCreatedAsc(tt.jobs)
			for i := 0; i < len(result); i++ {
				s.Equalf(tt.expectedJobNamesInList[i], result[i].GetName(), "result index %d: %s != %s", i, tt.expectedJobNamesInList[i], result[i].GetName())
			}
		})
	}
}

func (s *testSuite) Test_sortJobsByCreatedDesc() {
	j1 := createRadixJob("rj2", getPtr(s.timeNow.Add(time.Hour*2)))
	j2 := createRadixJob("rj2", getPtr(s.timeNow.Add(time.Hour)))
	fmt.Println(j1)
	fmt.Println(j2)
	tests := []listScenario{
		{
			name: "asc order",
			jobs: []v1.RadixJob{
				*createRadixJob("rj1", getPtr(s.timeNow)),
				*createRadixJob("rj2", getPtr(s.timeNow.Add(time.Hour))),
				*createRadixJob("rj3", getPtr(s.timeNow.Add(time.Hour*2))),
			},
			expectedJobNamesInList: []string{"rj3", "rj2", "rj1"},
		},
		{
			name: "desc",
			jobs: []v1.RadixJob{
				*createRadixJob("rj1", getPtr(s.timeNow.Add(time.Hour*2))),
				*createRadixJob("rj2", getPtr(s.timeNow.Add(time.Hour))),
				*createRadixJob("rj3", getPtr(s.timeNow)),
			},
			expectedJobNamesInList: []string{"rj1", "rj2", "rj3"},
		},
		{
			name: "random",
			jobs: []v1.RadixJob{
				*createRadixJob("rj1", getPtr(s.timeNow.Add(time.Hour))),
				*createRadixJob("rj2", getPtr(s.timeNow)),
				*createRadixJob("rj3", getPtr(s.timeNow.Add(time.Hour*2))),
			},
			expectedJobNamesInList: []string{"rj3", "rj1", "rj2"},
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := sortRadixJobsByCreatedDesc(tt.jobs)
			for i := 0; i < len(result); i++ {
				s.Equalf(tt.expectedJobNamesInList[i], result[i].GetName(), "result index %d: %s != %s", i, tt.expectedJobNamesInList[i], result[i].GetName())
			}
		})
	}
}

func getPtr(t time.Time) *time.Time {
	return &t
}

func createRadixJob(name string, created *time.Time) *v1.RadixJob {
	var createdStatus *metav1.Time
	if created != nil {
		createdStatus = &metav1.Time{Time: *created}
	}
	return &v1.RadixJob{ObjectMeta: metav1.ObjectMeta{Name: name}, Status: v1.RadixJobStatus{Created: createdStatus}}
}
