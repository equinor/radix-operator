package radixconfig_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/radixconfig"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

const (
	appName      = "anyapp"
	mutatedLabel = "mutated-by-webhook"
)

func writeVerbs(client *radixfake.Clientset) []string {
	var verbs []string
	for _, action := range client.Actions() {
		if action.GetResource().Resource != "radixapplications" {
			continue
		}
		switch action.GetVerb() {
		case "create", "update", "patch", "delete":
			verbs = append(verbs, action.GetVerb())
		}
	}
	return verbs
}

func newRA(annotations map[string]string, env string) *radixv1.RadixApplication {
	ra := utils.NewRadixApplicationBuilder().WithAppName(appName).WithEnvironment(env, "main").BuildRA()
	ra.SetAnnotations(annotations)
	return ra
}

func Test_ApplyRadixApplication(t *testing.T) {
	tests := map[string]struct {
		existingRA          *radixv1.RadixApplication
		newRA               *radixv1.RadixApplication
		dryRun              bool
		expectedOperation   radixconfig.ApplyRadixApplicationOperationResult
		expectedWriteVerbs  []string
		expectedAnnotations map[string]string
	}{
		"created when missing": {
			newRA:              newRA(nil, "dev"),
			expectedOperation:  radixconfig.ApplyRadixApplicationOperationResultCreated,
			expectedWriteVerbs: []string{"create"},
		},
		"unchanged when spec and annotations are equal does not write": {
			existingRA:         newRA(map[string]string{"a": "1"}, "dev"),
			newRA:              newRA(map[string]string{"a": "1"}, "dev"),
			expectedOperation:  radixconfig.ApplyRadixApplicationOperationResultUnchanged,
			expectedWriteVerbs: nil,
		},
		"updated when spec differs": {
			existingRA:         newRA(nil, "dev"),
			newRA:              newRA(nil, "qa"),
			expectedOperation:  radixconfig.ApplyRadixApplicationOperationResultUpdated,
			expectedWriteVerbs: []string{"update"},
		},
		"updated when only annotations differ": {
			existingRA:          newRA(map[string]string{"a": "1"}, "dev"),
			newRA:               newRA(map[string]string{"a": "2"}, "dev"),
			expectedOperation:   radixconfig.ApplyRadixApplicationOperationResultUpdated,
			expectedWriteVerbs:  []string{"update"},
			expectedAnnotations: map[string]string{"a": "2"},
		},
		"updated when annotations are added": {
			existingRA:          newRA(nil, "dev"),
			newRA:               newRA(map[string]string{"a": "1"}, "dev"),
			expectedOperation:   radixconfig.ApplyRadixApplicationOperationResultUpdated,
			expectedWriteVerbs:  []string{"update"},
			expectedAnnotations: map[string]string{"a": "1"},
		},
		"dry run of unchanged still sends update": {
			existingRA:         newRA(nil, "dev"),
			newRA:              newRA(nil, "dev"),
			dryRun:             true,
			expectedOperation:  radixconfig.ApplyRadixApplicationOperationResultUpdated,
			expectedWriteVerbs: []string{"update"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			client := radixfake.NewSimpleClientset() //nolint:staticcheck // SA1019: Ignore linting deprecated fields
			if test.existingRA != nil {
				_, err := client.RadixV1().RadixApplications(utils.GetAppNamespace(appName)).Create(context.Background(), test.existingRA, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			client.ClearActions()

			result, operation, err := radixconfig.ApplyRadixApplication(context.Background(), client, test.newRA, test.dryRun)
			require.NoError(t, err)
			assert.Equal(t, test.expectedOperation, operation)
			assert.Equal(t, test.expectedWriteVerbs, writeVerbs(client))
			require.NotNil(t, result)
			assert.Equal(t, test.newRA.Name, result.Name)
			if test.expectedAnnotations != nil {
				assert.Equal(t, test.expectedAnnotations, result.GetAnnotations())
			}
		})
	}
}

func Test_ApplyRadixApplication_ReturnsMutatedResponse(t *testing.T) {
	tests := map[string]struct {
		verb       string
		existingRA *radixv1.RadixApplication
		newRA      *radixv1.RadixApplication
	}{
		"create": {verb: "create", newRA: newRA(nil, "dev")},
		"update": {verb: "update", existingRA: newRA(nil, "dev"), newRA: newRA(map[string]string{"a": "1"}, "dev")},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			client := radixfake.NewSimpleClientset() //nolint:staticcheck // SA1019: Ignore linting deprecated fields
			if test.existingRA != nil {
				_, err := client.RadixV1().RadixApplications(utils.GetAppNamespace(appName)).Create(context.Background(), test.existingRA, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			client.PrependReactor(test.verb, "radixapplications", func(action k8stesting.Action) (bool, runtime.Object, error) {
				mutated := action.(k8stesting.CreateAction).GetObject().(*radixv1.RadixApplication).DeepCopy()
				mutated.Labels = map[string]string{mutatedLabel: "true"}
				return true, mutated, nil
			})

			result, _, err := radixconfig.ApplyRadixApplication(context.Background(), client, test.newRA, false)
			require.NoError(t, err)
			assert.Equal(t, map[string]string{mutatedLabel: "true"}, result.Labels)
			assert.Nil(t, test.newRA.Labels, "input must not be modified")
		})
	}
}

func Test_ApplyRadixApplication_DryRunOptions(t *testing.T) {
	tests := map[string]struct {
		existing bool
		dryRun   bool
	}{
		"create":         {},
		"dry-run create": {dryRun: true},
		"update":         {existing: true},
		"dry-run update": {existing: true, dryRun: true},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			client := radixfake.NewSimpleClientset() //nolint:staticcheck // SA1019: Ignore linting deprecated fields
			if test.existing {
				_, err := client.RadixV1().RadixApplications(utils.GetAppNamespace(appName)).Create(context.Background(), newRA(nil, "dev"), metav1.CreateOptions{})
				require.NoError(t, err)
			}
			client.ClearActions()

			_, _, err := radixconfig.ApplyRadixApplication(context.Background(), client, newRA(nil, "qa"), test.dryRun)
			require.NoError(t, err)
			require.Len(t, writeVerbs(client), 1)
			var expected []string
			if test.dryRun {
				expected = []string{metav1.DryRunAll}
			}
			for _, action := range client.Actions() {
				switch action.GetVerb() {
				case "create":
					options := action.(interface{ GetCreateOptions() metav1.CreateOptions }).GetCreateOptions()
					assert.Equal(t, expected, options.DryRun)
				case "update":
					options := action.(interface{ GetUpdateOptions() metav1.UpdateOptions }).GetUpdateOptions()
					assert.Equal(t, expected, options.DryRun)
				}
			}
		})
	}
}
