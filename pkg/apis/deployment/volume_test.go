package deployment

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_EmptyDir(t *testing.T) {
	appName, envName, compName := "anyapp", "anyenv", "anycomp"

	tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	builder := utils.NewDeploymentBuilder().
		WithRadixApplication(utils.NewRadixApplicationBuilder().WithAppName(appName).WithRadixRegistration(utils.NewRegistrationBuilder().WithName(appName))).
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponents(
			utils.NewDeployComponentBuilder().WithName(compName).WithVolumeMounts(
				v1.RadixVolumeMount{Name: "cache", Path: "/cache", EmptyDir: &v1.RadixEmptyDirVolumeMount{SizeLimit: resource.MustParse("50M")}},
				v1.RadixVolumeMount{Name: "log", Path: "/log", EmptyDir: &v1.RadixEmptyDirVolumeMount{SizeLimit: resource.MustParse("100M")}},
			),
		)

	rd, err := ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, builder)
	require.NoError(t, err)
	assert.NotNil(t, rd)

	deployment, err := kubeclient.AppsV1().Deployments(utils.GetEnvironmentNamespace(appName, envName)).Get(context.Background(), compName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Len(t, deployment.Spec.Template.Spec.Containers[0].VolumeMounts, 2)
	assert.Len(t, deployment.Spec.Template.Spec.Volumes, 2)
}
