package batch

import (
	"testing"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

func Test_getJobImage(t *testing.T) {
	type args struct {
		jobComponentImage string
		imageTagName      string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "image without repo and tag, no imageTagName", args: args{jobComponentImage: "alpine", imageTagName: ""}, want: "alpine"},
		{name: "image with repo, no tag, no imageTagName", args: args{jobComponentImage: "somerepo/alpine", imageTagName: ""}, want: "somerepo/alpine"},
		{name: "image with repo, org, no tag, no imageTagName", args: args{jobComponentImage: "somerepo/org/alpine", imageTagName: ""}, want: "somerepo/org/alpine"},
		{name: "image with repo and domain, org, no tag, no imageTagName", args: args{jobComponentImage: "somerepo.com/org/alpine", imageTagName: ""}, want: "somerepo/org/alpine"},
		{name: "image without repo and tag, no imageTagName", args: args{jobComponentImage: "alpine", imageTagName: ""}, want: "alpine"},
		{name: "image with repo and tag, no imageTagName", args: args{jobComponentImage: "somerepo/alpine", imageTagName: ""}, want: "somerepo/alpine"},
		{name: "image with repo, org and tag, no imageTagName", args: args{jobComponentImage: "somerepo/org/alpine", imageTagName: ""}, want: "somerepo/org/alpine"},
		{name: "image with repo and domain, org and tag, no imageTagName", args: args{jobComponentImage: "somerepo.com/org/alpine", imageTagName: ""}, want: "somerepo/org/alpine"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobComponent := radixv1.RadixDeployJobComponent{Image: tt.args.jobComponentImage}
			radixBatch := radixv1.RadixBatchJob{ImageTagName: tt.args.imageTagName}
			if gotImage := getJobImage(&jobComponent, &radixBatch); gotImage != tt.want {
				t.Errorf("getJobImage() = %v, want %v", gotImage, tt.want)
			}
		})
	}
}
