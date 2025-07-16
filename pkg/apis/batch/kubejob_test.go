package batch

import (
	"context"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
	"testing"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

func Test_getJobImage(t *testing.T) {
	type args struct {
		jobComponentImage string
		imageTagName      string
		jobImage          string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "image without repo and tag, no imageTagName", args: args{jobComponentImage: "alpine", imageTagName: ""}, want: "alpine"},
		{name: "image with repo, no tag, no imageTagName", args: args{jobComponentImage: "somerepo/alpine", imageTagName: ""}, want: "somerepo/alpine"},
		{name: "image with repo, org, no tag, no imageTagName", args: args{jobComponentImage: "somerepo/org/alpine", imageTagName: ""}, want: "somerepo/org/alpine"},
		{name: "image with repo and domain, org, no tag, no imageTagName", args: args{jobComponentImage: "somerepo.com/org/alpine", imageTagName: ""}, want: "somerepo.com/org/alpine"},
		{name: "image without repo and tag, no imageTagName", args: args{jobComponentImage: "alpine:abc", imageTagName: ""}, want: "alpine:abc"},
		{name: "image with repo and tag, no imageTagName", args: args{jobComponentImage: "somerepo/alpine:abc", imageTagName: ""}, want: "somerepo/alpine:abc"},
		{name: "image with repo, org and tag, no imageTagName", args: args{jobComponentImage: "somerepo/org/alpine:abc", imageTagName: ""}, want: "somerepo/org/alpine:abc"},
		{name: "image with repo and domain, org and tag, no imageTagName", args: args{jobComponentImage: "somerepo.com/org/alpine:abc", imageTagName: ""}, want: "somerepo.com/org/alpine:abc"},
		{name: "image without repo and tag, with imageTagName", args: args{jobComponentImage: "alpine", imageTagName: "dfe"}, want: "alpine:dfe"},
		{name: "image with repo, no tag, with imageTagName", args: args{jobComponentImage: "somerepo/alpine", imageTagName: "dfe"}, want: "somerepo/alpine:dfe"},
		{name: "image with repo, org, no tag, with imageTagName", args: args{jobComponentImage: "somerepo/org/alpine", imageTagName: "dfe"}, want: "somerepo/org/alpine:dfe"},
		{name: "image with repo and domain, org, no tag, with imageTagName", args: args{jobComponentImage: "somerepo.com/org/alpine", imageTagName: "dfe"}, want: "somerepo.com/org/alpine:dfe"},
		{name: "image without repo and tag, with imageTagName", args: args{jobComponentImage: "alpine:abc", imageTagName: "dfe"}, want: "alpine:dfe"},
		{name: "image with repo and tag, with imageTagName", args: args{jobComponentImage: "somerepo/alpine:abc", imageTagName: "dfe"}, want: "somerepo/alpine:dfe"},
		{name: "image with repo, org and tag, with imageTagName", args: args{jobComponentImage: "somerepo/org/alpine:abc", imageTagName: "dfe"}, want: "somerepo/org/alpine:dfe"},
		{name: "image with repo and domain, org and tag, with imageTagName", args: args{jobComponentImage: "somerepo.com/org/alpine:abc", imageTagName: "dfe"}, want: "somerepo.com/org/alpine:dfe"},
		{name: "image with image, no imageTagName", args: args{jobComponentImage: "somerepo.com/org/alpine:abc", jobImage: "otherrepo.com/org/alpine:abc"}, want: "otherrepo.com/org/alpine:abc"},
		{name: "image with image, with imageTagName", args: args{jobComponentImage: "somerepo.com/org/alpine:abc", jobImage: "otherrepo.com/org/alpine:abc", imageTagName: "dfe"}, want: "otherrepo.com/org/alpine:dfe"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobComponent := radixv1.RadixDeployJobComponent{Image: tt.args.jobComponentImage}
			radixBatch := radixv1.RadixBatchJob{ImageTagName: tt.args.imageTagName, Image: tt.args.jobImage}
			if gotImage := getJobImage(&jobComponent, &radixBatch); gotImage != tt.want {
				t.Errorf("getJobImage() = %v, want %v", gotImage, tt.want)
			}
		})
	}
}

func Test_Variables(t *testing.T) {
	type args struct {
		jobComponentVariables map[string]string
		jobVariables          map[string]string
	}
	const jobName1 = "job1"
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{name: "no component and image env vars", args: args{}, want: map[string]string{defaults.RadixScheduleJobNameEnvironmentVariable: jobName1}},
		{name: "component env vars, no image env vars", args: args{
			jobComponentVariables: map[string]string{"VAR1": "value1", "VAR2": "value2"},
		}, want: map[string]string{defaults.RadixScheduleJobNameEnvironmentVariable: jobName1, "VAR1": "value1", "VAR2": "value2"}},
		{name: "no component env vars, image env vars", args: args{
			jobVariables: map[string]string{"VAR1": "value1", "VAR2": "value2"},
		}, want: map[string]string{defaults.RadixScheduleJobNameEnvironmentVariable: jobName1, "VAR1": "value1", "VAR2": "value2"}},
		{name: "component env vars, image env vars", args: args{
			jobComponentVariables: map[string]string{"VAR1": "value1", "VAR2": "value2"},
			jobVariables:          map[string]string{"VAR2": "value11", "VAR3": "value3"},
		}, want: map[string]string{defaults.RadixScheduleJobNameEnvironmentVariable: jobName1, "VAR1": "value1", "VAR2": "value11", "VAR3": "value3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobComponent := radixv1.RadixDeployJobComponent{EnvironmentVariables: tt.args.jobComponentVariables}
			radixBatchJob := radixv1.RadixBatchJob{Variables: tt.args.jobVariables}
			kubeUtil, err := kube.New(fake.NewSimpleClientset(), nil, nil, nil)
			require.NoError(t, err, "should not return error when creating kubeUtil")
			s := syncer{kubeUtil: kubeUtil}
			envVars, err := s.getContainerEnvironmentVariables(context.Background(), &radixv1.RadixDeployment{}, &jobComponent, &radixBatchJob, jobName1)
			require.NoError(t, err, "should not return error when getting environment variables")
			if assert.Len(t, envVars, len(tt.want), "should return expected number of environment variables") {
				for _, envVar := range envVars {
					assert.Equal(t, tt.want[envVar.Name], envVar.Value, "should return expected environment variable for key %s", envVar.Name)
				}
			}
		})
	}
}
