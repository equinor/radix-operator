package tasks_test

import (
	"context"
	"testing"
	"time"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/equinor/radix-operator/radix-operator/scheduler/tasks"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

func setupTest(t *testing.T) *kube.Kube {
	kubeClient := kubefake.NewSimpleClientset()
	radixClient := radixfake.NewSimpleClientset()
	kedaClient := kedafake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	kubeUtil, _ := kube.New(kubeClient, radixClient, kedaClient, secretproviderclient)
	return kubeUtil
}

type envProps struct {
	name              string
	orphaned          bool
	orphanedTimestamp string
}

func TestCleanupRadixEnvironments(t *testing.T) {
	type scenario struct {
		name                 string
		existingEnvironments []envProps
		expectedEnvironments []envProps
	}
	const (
		retentionPeriod = time.Minute * 10
		env1            = "env1"
		env2            = "env2"
	)
	scenarios := []scenario{
		{
			name:                 "No ens",
			existingEnvironments: []envProps{},
			expectedEnvironments: []envProps{},
		},
		{
			name:                 "Not orphaned environments",
			existingEnvironments: []envProps{{name: env1}},
			expectedEnvironments: []envProps{{name: env1}},
		},
	}
	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			kubeUtil := setupTest(t)
			task := tasks.NewRadixEnvironmentsCleanup(context.Background(), kubeUtil, retentionPeriod)
			for _, envProp := range ts.existingEnvironments {
				_, err := kubeUtil.RadixClient().RadixV1().RadixEnvironments().Create(context.Background(), createRadixEnvironment(envProp), metav1.CreateOptions{})
				require.NoError(t, err, "Failed to create existing RadixEnvironment %s", envProp.name)
			}
			task.Run()
			environmentList, err := kubeUtil.RadixClient().RadixV1().RadixEnvironments().List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			assert.Len(t, environmentList.Items, len(ts.expectedEnvironments), "Mismatch expected environment list")
			if len(environmentList.Items) != len(ts.expectedEnvironments) {
				return
			}
			actualEnvMap := convertToEnvMap(environmentList.Items)
			for _, expectedEnv := range ts.expectedEnvironments {
				actualEnv, ok := actualEnvMap[expectedEnv.name]
				assert.True(t, ok, "Missing the environment %s", expectedEnv.name)
				if !ok {
					continue
				}
				assert.Equal(t, expectedEnv.orphaned, actualEnv.Status.Orphaned, "Invalid orphaned value for env %s", expectedEnv.name)
				assert.Equal(t, expectedEnv.orphanedTimestamp, actualEnv.Status.OrphanedTimestamp, "Invalid orphaned timestamp for env %s", expectedEnv.name)
			}
		})
	}
}

func convertToEnvMap(radixEnvironments []radixv1.RadixEnvironment) map[string]radixv1.RadixEnvironment {
	return slice.Reduce(radixEnvironments, make(map[string]radixv1.RadixEnvironment), func(acc map[string]radixv1.RadixEnvironment, env radixv1.RadixEnvironment) map[string]radixv1.RadixEnvironment {
		acc[env.Spec.EnvName] = env
		return acc
	})
}

func createRadixEnvironment(prop envProps) *radixv1.RadixEnvironment {
	appName := utils.RandString(5)
	return &radixv1.RadixEnvironment{
		ObjectMeta: metav1.ObjectMeta{Name: appName},
		Spec:       radixv1.RadixEnvironmentSpec{AppName: appName, EnvName: prop.name},
		Status: radixv1.RadixEnvironmentStatus{
			Orphaned:          prop.orphaned,
			OrphanedTimestamp: prop.orphanedTimestamp,
		},
	}
}
