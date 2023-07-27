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
