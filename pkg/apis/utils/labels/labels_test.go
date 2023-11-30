package labels

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	kubelabels "k8s.io/apimachinery/pkg/labels"
)

func Test_Merge(t *testing.T) {
	actual := Merge(
		kubelabels.Set{"a": "a", "b": "b", "c": "c1"},
		kubelabels.Set{"a": "a", "c": "c2", "d": "d"},
	)
	expected := kubelabels.Set{"a": "a", "b": "b", "c": "c2", "d": "d"}
	assert.Equal(t, expected, actual)
}

func Test_ForApplicationName(t *testing.T) {
	actual := ForApplicationName("anyappname")
	expected := kubelabels.Set{kube.RadixAppLabel: "anyappname"}
	assert.Equal(t, expected, actual)
}

func Test_ForComponentName(t *testing.T) {
	actual := ForComponentName("anycomponentname")
	expected := kubelabels.Set{kube.RadixComponentLabel: "anycomponentname"}
	assert.Equal(t, expected, actual)
}

func Test_ForCommitId(t *testing.T) {
	actual := ForCommitId("anycommit")
	expected := kubelabels.Set{kube.RadixCommitLabel: "anycommit"}
	assert.Equal(t, expected, actual)
}

func Test_ForPodIsJobScheduler(t *testing.T) {
	actual := ForPodIsJobScheduler()
	expected := kubelabels.Set{kube.RadixPodIsJobSchedulerLabel: "true"}
	assert.Equal(t, expected, actual)
}

func Test_ForServiceAccountWithRadixIdentity(t *testing.T) {
	actual := ForServiceAccountWithRadixIdentity(nil)
	assert.Equal(t, kubelabels.Set(nil), actual)

	actual = ForServiceAccountWithRadixIdentity(&v1.Identity{})
	assert.Equal(t, kubelabels.Set(nil), actual)

	actual = ForServiceAccountWithRadixIdentity(&v1.Identity{Azure: &v1.AzureIdentity{ClientId: "any"}})
	expected := kubelabels.Set{"azure.workload.identity/use": "true"}
	assert.Equal(t, expected, actual)
}

func Test_ForPodWithRadixIdentity(t *testing.T) {
	actual := ForPodWithRadixIdentity(nil)
	assert.Equal(t, kubelabels.Set(nil), actual)

	actual = ForPodWithRadixIdentity(&v1.Identity{})
	assert.Equal(t, kubelabels.Set(nil), actual)

	actual = ForPodWithRadixIdentity(&v1.Identity{Azure: &v1.AzureIdentity{ClientId: "any"}})
	expected := kubelabels.Set{"azure.workload.identity/use": "true"}
	assert.Equal(t, expected, actual)
}

func Test_ForBatchType(t *testing.T) {
	actual := ForBatchType(kube.RadixBatchTypeBatch)
	expected := kubelabels.Set{kube.RadixBatchTypeLabel: string(kube.RadixBatchTypeBatch)}
	assert.Equal(t, expected, actual)

	actual = ForBatchType(kube.RadixBatchTypeJob)
	expected = kubelabels.Set{kube.RadixBatchTypeLabel: string(kube.RadixBatchTypeJob)}
	assert.Equal(t, expected, actual)
}

func Test_ForForBatchName(t *testing.T) {
	actual := ForBatchName("anyname")
	expected := kubelabels.Set{kube.RadixBatchNameLabel: "anyname"}
	assert.Equal(t, expected, actual)
}

func Test_ForBatchJobName(t *testing.T) {
	actual := ForBatchJobName("anyjobname")
	expected := kubelabels.Set{kube.RadixBatchJobNameLabel: "anyjobname"}
	assert.Equal(t, expected, actual)
}

func Test_ForJobType(t *testing.T) {
	actual := ForJobType("anyjobtype")
	expected := kubelabels.Set{kube.RadixJobTypeLabel: "anyjobtype"}
	assert.Equal(t, expected, actual)
}

func Test_ForBatchScheduleJobType(t *testing.T) {
	actual := ForBatchScheduleJobType()
	expected := kubelabels.Set{kube.RadixJobTypeLabel: kube.RadixJobTypeBatchSchedule}
	assert.Equal(t, expected, actual)
}

func Test_ForJobScheduleJobType(t *testing.T) {
	actual := ForJobScheduleJobType()
	expected := kubelabels.Set{kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule}
	assert.Equal(t, expected, actual)
}

func Test_ForAccessValidation(t *testing.T) {
	actual := ForAccessValidation()
	expected := kubelabels.Set{kube.RadixAccessValidationLabel: "true"}
	assert.Equal(t, expected, actual)
}

func Test_ForPipelineJobName(t *testing.T) {
	actual := ForPipelineJobName("anypipelinejobname")
	expected := kubelabels.Set{kube.RadixJobNameLabel: "anypipelinejobname"}
	assert.Equal(t, expected, actual)
}

func Test_ForPipelineJobType(t *testing.T) {
	actual := ForPipelineJobType()
	expected := kubelabels.Set{kube.RadixJobTypeLabel: kube.RadixJobTypeJob}
	assert.Equal(t, expected, actual)
}

func Test_ForPipelineJobPipelineType(t *testing.T) {
	actual := ForPipelineJobPipelineType("anypipelinetype")
	expected := kubelabels.Set{kube.RadixPipelineTypeLabels: "anypipelinetype"}
	assert.Equal(t, expected, actual)
}

func Test_ForRadixImageTag(t *testing.T) {
	actual := ForRadixImageTag("anyimagetag")
	expected := kubelabels.Set{kube.RadixImageTagLabel: "anyimagetag"}
	assert.Equal(t, expected, actual)
}

func Test_ForDNSAlias(t *testing.T) {
	actual := ForDNSAlias()
	expected := kubelabels.Set{kube.RadixAliasLabel: "true"}
	assert.Equal(t, expected, actual)
}

func Test_RequirementRadixBatchNameLabelExists(t *testing.T) {
	actual := requirementRadixBatchNameLabelExists()
	expected := kubelabels.Set{kube.RadixBatchNameLabel: "anyname"}
	assert.True(t, actual.Matches(expected))
}

func TestGetRadixBatchDescendantsSelector(t *testing.T) {
	type args struct {
		componentName string
		labels        kubelabels.Set
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "No labels", args: args{componentName: "anycomponentname", labels: kubelabels.Set{}}, want: false},
		{name: "Wrong component name", args: args{componentName: "different-comp",
			labels: Merge(ForComponentName("comp1"), ForJobScheduleJobType(), ForBatchName("somebatch"))},
			want: false},
		{name: "No batch name", args: args{componentName: "comp1",
			labels: Merge(ForComponentName("comp1"), ForJobScheduleJobType())},
			want: false},
		{name: "No job type job schedule", args: args{componentName: "different-comp",
			labels: Merge(ForComponentName("comp1"), ForBatchName("somebatch"))},
			want: false},
		{name: "Wrong job type job schedule", args: args{componentName: "different-comp",
			labels: Merge(ForComponentName("comp1"), ForBatchName("somebatch"), kubelabels.Set{
				kube.RadixJobTypeLabel: "other-type"})},
			want: false},
		{name: "Correct component name and all labels exist", args: args{componentName: "comp1",
			labels: Merge(ForComponentName("comp1"), ForJobScheduleJobType(), ForBatchName("somebatch"))},
			want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, GetRadixBatchDescendantsSelector(tt.args.componentName).Matches(tt.args.labels), "GetRadixBatchDescendantsSelector(%v)", tt.args.componentName)
		})
	}
}
