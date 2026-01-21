package labels

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"
)

func Test_Merge(t *testing.T) {
	actual := Merge(
		kubelabels.Set{"a": "a", "b": "b", "c": "c1"},
		kubelabels.Set{"a": "a", "c": "c2", "d": "d"},
		kubelabels.Set(nil),
	)
	expected := kubelabels.Set{"a": "a", "b": "b", "c": "c2", "d": "d"}
	assert.Equal(t, expected, actual)
}

func Test_ForApplicationName(t *testing.T) {
	actual := ForApplicationName("anyappname")
	expected := kubelabels.Set{kube.RadixAppLabel: "anyappname"}
	assert.Equal(t, expected, actual)
}
func Test_ForApplicationID(t *testing.T) {
	appId := radixv1.ULID{ULID: ulid.Make()}
	actual := ForApplicationID(appId)
	expected := kubelabels.Set{kube.RadixAppIDLabel: appId.String()}
	assert.Equal(t, expected, actual)
}
func Test_ForApplicationID_WhenZero(t *testing.T) {
	actual := ForApplicationID(radixv1.ULID{ULID: ulid.Zero})
	assert.Nil(t, actual)
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

func Test_ForOAuthProxyPodWithRadixIdentityWithWorkloadIdentity(t *testing.T) {
	actual := ForOAuthProxyPodWithRadixIdentity(nil)
	assert.Equal(t, kubelabels.Set(nil), actual, "Not expected labels when there is no OAuth2")

	actual = ForOAuthProxyPodWithRadixIdentity(&radixv1.OAuth2{})
	assert.Equal(t, kubelabels.Set(nil), actual, "Not expected labels when there is no Credentials")

	actual = ForOAuthProxyPodWithRadixIdentity(&radixv1.OAuth2{Credentials: radixv1.Secret, ClientID: "any-client-id"})
	assert.Equal(t, kubelabels.Set(nil), actual, "Not expected labels when Credentials is Secret")

	actual = ForOAuthProxyPodWithRadixIdentity(&radixv1.OAuth2{Credentials: radixv1.AzureWorkloadIdentity, ClientID: "any-client-id"})
	expected := kubelabels.Set{"azure.workload.identity/use": "true"}
	assert.Equal(t, expected, actual, "Expected labels when Credentials is AzureWorkloadIdentity")
}

func Test_ForPodWithRadixIdentity(t *testing.T) {
	actual := ForPodWithRadixIdentity(nil)
	assert.Equal(t, kubelabels.Set(nil), actual)

	actual = ForPodWithRadixIdentity(&radixv1.Identity{})
	assert.Equal(t, kubelabels.Set(nil), actual)

	actual = ForPodWithRadixIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "any"}})
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

func Test_ForDNSAliasComponentIngress(t *testing.T) {
	alias := &radixv1.RadixDNSAlias{
		ObjectMeta: v1.ObjectMeta{Name: "any-dns-alias"},
		Spec: radixv1.RadixDNSAliasSpec{
			AppName:   "any-app",
			Component: "any-component",
		},
	}
	actual := ForDNSAliasComponentIngress(alias)
	expected := kubelabels.Set{kube.RadixAliasLabel: alias.Name, kube.RadixAppLabel: alias.Spec.AppName, kube.RadixComponentLabel: alias.Spec.Component}
	assert.Equal(t, expected, actual)
}

func Test_ForDNSAliasOAuthIngress(t *testing.T) {
	alias := &radixv1.RadixDNSAlias{
		ObjectMeta: v1.ObjectMeta{Name: "any-dns-alias"},
		Spec: radixv1.RadixDNSAliasSpec{
			AppName:   "any-app",
			Component: "any-component",
		},
	}
	actual := ForDNSAliasOAuthIngress(alias)
	expected := kubelabels.Set{kube.RadixAliasLabel: alias.Name, kube.RadixAppLabel: alias.Spec.AppName, kube.RadixAuxiliaryComponentLabel: alias.Spec.Component, kube.RadixAuxiliaryComponentTypeLabel: radixv1.OAuthProxyAuxiliaryComponentType}
	assert.Equal(t, expected, actual)
}

func Test_ForDNSAliasRbac(t *testing.T) {
	actual := ForDNSAliasRbac("any-app")
	expected := kubelabels.Set{kube.RadixAppLabel: "any-app", kube.RadixAliasLabel: "true"}
	assert.Equal(t, expected, actual)
}

func Test_ForExternalDNSTLSSecret(t *testing.T) {
	actual := ForExternalDNSTLSSecret("any-app", radixv1.RadixDeployExternalDNS{FQDN: "test.com"})
	expected := kubelabels.Set{kube.RadixAppLabel: "any-app", kube.RadixExternalAliasFQDNLabel: "test.com"}
	assert.Equal(t, expected, actual)
}

func Test_ForExternalDNSCertificate(t *testing.T) {
	actual := ForExternalDNSCertificate("any-app", radixv1.RadixDeployExternalDNS{FQDN: "test.com"})
	expected := kubelabels.Set{kube.RadixAppLabel: "any-app", kube.RadixExternalAliasFQDNLabel: "test.com"}
	assert.Equal(t, expected, actual)
}

func Test_ForBlobCSIAzurePersistentVolume(t *testing.T) {
	actual := ForBlobCSIAzurePersistentVolume("any-app", "any-ns", "any-comp", radixv1.RadixVolumeMount{Name: "any-vol"})
	expected := kubelabels.Set{
		kube.RadixAppLabel:             "any-app",
		kube.RadixNamespace:            "any-ns",
		kube.RadixComponentLabel:       "any-comp",
		kube.RadixVolumeMountNameLabel: "any-vol",
	}
	assert.Equal(t, expected, actual)
}

func Test_ForBlobCSIAzurePersistentVolumeClaim(t *testing.T) {
	actual := ForBlobCSIAzurePersistentVolumeClaim("any-app", "any-comp", radixv1.RadixVolumeMount{Name: "any-vol"})
	expected := kubelabels.Set{
		kube.RadixAppLabel:             "any-app",
		kube.RadixComponentLabel:       "any-comp",
		kube.RadixVolumeMountNameLabel: "any-vol",
		kube.RadixMountTypeLabel:       "unsupported",
	}
	assert.Equal(t, expected, actual)

	actual = ForBlobCSIAzurePersistentVolumeClaim("any-app", "any-comp", radixv1.RadixVolumeMount{Name: "any-vol", Type: "any-type"})
	expected = kubelabels.Set{
		kube.RadixAppLabel:             "any-app",
		kube.RadixComponentLabel:       "any-comp",
		kube.RadixVolumeMountNameLabel: "any-vol",
		kube.RadixMountTypeLabel:       "any-type",
	}
	assert.Equal(t, expected, actual)

	actual = ForBlobCSIAzurePersistentVolumeClaim("any-app", "any-comp", radixv1.RadixVolumeMount{Name: "any-vol", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}})
	expected = kubelabels.Set{
		kube.RadixAppLabel:             "any-app",
		kube.RadixComponentLabel:       "any-comp",
		kube.RadixVolumeMountNameLabel: "any-vol",
		kube.RadixMountTypeLabel:       string(radixv1.MountTypeBlobFuse2Fuse2CsiAzure),
	}
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
