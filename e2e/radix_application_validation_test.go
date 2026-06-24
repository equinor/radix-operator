package e2e

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// createNamespaceForTest creates a namespace for testing and returns a cleanup function
func createNamespaceForTest(t *testing.T, c client.Client, appName string) func() {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: appName + "-app",
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
			},
		},
	}
	err := c.Create(context.Background(), ns)
	require.NoError(t, err)

	return func() {
		_ = c.Delete(context.Background(), ns)
	}
}

// createRadixRegistrationAndNamespaceForTest creates a RadixRegistration and its namespace for testing
// appName: the name of the application (used for RadixRegistration name and as base for namespace)
// Returns cleanup function and the app namespace name
func createRadixRegistrationAndNamespaceForTest(t *testing.T, c client.Client, appName string) (cleanup func(), appNamespace string) {
	appNamespace = appName + "-app"
	nsCleanup := createNamespaceForTest(t, c, appName) // Reuse the cleanup function for namespace

	// Create RadixRegistration
	rr := &v1.RadixRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: appName,
		},
		Spec: v1.RadixRegistrationSpec{
			CloneURL:          "git@github.com:equinor/" + appName + ".git",
			AdGroups:          []string{"test-group"},
			ConfigBranch:      "main",
			ConfigurationItem: "test-item",
		},
	}
	err := c.Create(t.Context(), rr)
	require.NoError(t, err)

	return func() {
		_ = c.Delete(t.Context(), rr)
		nsCleanup()
	}, appNamespace
}

// TestRadixApplicationWebhookSmokeTest tests that the webhook is working by verifying createRRExistValidator
func TestRadixApplicationWebhookSmokeTest(t *testing.T) {
	c := getClient(t)

	t.Run("rejects RadixApplication when RadixRegistration does not exist", func(t *testing.T) {
		ra := &v1.RadixApplication{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nonexistent-app",
				Namespace: "nonexistent-app",
			},
			Spec: v1.RadixApplicationSpec{
				Environments: []v1.Environment{
					{Name: "dev"},
				},
			},
		}

		err := c.Create(t.Context(), ra, client.DryRunAll)
		assert.Error(t, err, "Should reject RadixApplication without corresponding RadixRegistration")
		if err != nil {
			t.Logf("Got expected error: %v", err)
		}
	})

	t.Run("accepts RadixApplication when RadixRegistration exists", func(t *testing.T) {
		appName := "test-webhook-app"
		appNamespace := appName + "-app"

		// Create namespace first
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: appNamespace,
				Labels: map[string]string{
					kube.RadixAppLabel: appName,
				},
			},
		}
		err := c.Create(t.Context(), ns)
		require.NoError(t, err)
		defer func() { _ = c.Delete(t.Context(), ns) }()

		// Create the RadixRegistration
		rr := &v1.RadixRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: appName,
			},
			Spec: v1.RadixRegistrationSpec{
				CloneURL:          "git@github.com:equinor/" + appName + ".git",
				AdGroups:          []string{"test-group"},
				ConfigBranch:      "main",
				ConfigurationItem: "test-item",
			},
		}
		err = c.Create(t.Context(), rr)
		require.NoError(t, err)
		defer func() { _ = c.Delete(t.Context(), rr) }()

		// Now create RadixApplication with matching name
		ra := &v1.RadixApplication{
			ObjectMeta: metav1.ObjectMeta{
				Name:      appName,
				Namespace: appNamespace,
			},
			Spec: v1.RadixApplicationSpec{
				Environments: []v1.Environment{
					{Name: "dev"},
				},
			},
		}

		err = c.Create(t.Context(), ra, client.DryRunAll)
		assert.NoError(t, err, "Should accept RadixApplication when RadixRegistration exists")
	})
}

// TestRadixApplicationEnvironmentsValidation tests Environments field validation (MinItems=1)
func TestRadixApplicationEnvironmentsValidation(t *testing.T) {
	c := getClient(t)
	appName := "test-env-validation"
	cleanup, appNamespace := createRadixRegistrationAndNamespaceForTest(t, c, appName)
	defer cleanup()

	testCases := []struct {
		name         string
		environments []v1.Environment
		shouldError  bool
	}{
		{
			name: "valid - single environment",
			environments: []v1.Environment{
				{Name: "dev"},
			},
			shouldError: false,
		},
		{
			name: "valid - multiple environments",
			environments: []v1.Environment{
				{Name: "dev"},
				{Name: "prod"},
			},
			shouldError: false,
		},
		{
			name:         "invalid - empty environments list (MinItems=1)",
			environments: []v1.Environment{},
			shouldError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ra := &v1.RadixApplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      appName,
					Namespace: appNamespace,
				},
				Spec: v1.RadixApplicationSpec{
					Environments: tc.environments,
				},
			}

			err := c.Create(t.Context(), ra, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestRadixApplicationEnvironmentNameValidation tests Environment.Name validation (MinLength=1, MaxLength=63, Pattern)
func TestRadixApplicationEnvironmentNameValidation(t *testing.T) {
	c := getClient(t)
	appName := "test-env-name"
	cleanup, appNamespace := createRadixRegistrationAndNamespaceForTest(t, c, appName)
	defer cleanup()

	testCases := []struct {
		name        string
		envName     string
		shouldError bool
	}{
		{
			name:        "valid - lowercase alphanumeric",
			envName:     "dev",
			shouldError: false,
		},
		{
			name:        "valid - with hyphens",
			envName:     "qa-1",
			shouldError: false,
		},
		{
			name:        "valid - reasonable length",
			envName:     "production-environment",
			shouldError: false,
		},
		{
			name:        "invalid - empty string (MinLength=1)",
			envName:     "",
			shouldError: true,
		},
		{
			name:        "invalid - exceeds max length (64 chars)",
			envName:     "a2345678901234567890123456789012345678901234567890123456789012345",
			shouldError: true,
		},
		{
			name:        "invalid - uppercase letters",
			envName:     "Dev",
			shouldError: true,
		},
		{
			name:        "invalid - starts with hyphen",
			envName:     "-dev",
			shouldError: true,
		},
		{
			name:        "invalid - reserved name 'app' (CEL rule)",
			envName:     "app",
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ra := &v1.RadixApplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      appName,
					Namespace: appNamespace,
				},
				Spec: v1.RadixApplicationSpec{
					Environments: []v1.Environment{
						{Name: tc.envName},
					},
				},
			}

			err := c.Create(t.Context(), ra, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestRadixApplicationEnvBuildFromValidation tests EnvBuild.From field validation (MinLength=1, MaxLength=255)
func TestRadixApplicationEnvBuildFromValidation(t *testing.T) {
	c := getClient(t)
	appName := "test-envbuild-from"
	cleanup, appNamespace := createRadixRegistrationAndNamespaceForTest(t, c, appName)
	defer cleanup()

	testCases := []struct {
		name        string
		from        string
		shouldError bool
	}{
		{
			name:        "valid - simple branch name",
			from:        "main",
			shouldError: false,
		},
		{
			name:        "valid - feature branch",
			from:        "feature/test",
			shouldError: false,
		},
		{
			name:        "valid - max length (253 chars)",
			from:        "2345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234",
			shouldError: false,
		},
		{
			name:        "valid - empty (optional field)",
			from:        "",
			shouldError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ra := &v1.RadixApplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      appName,
					Namespace: appNamespace,
				},
				Spec: v1.RadixApplicationSpec{
					Environments: []v1.Environment{
						{
							Name: "dev",
							Build: v1.EnvBuild{
								From: tc.from,
							},
						},
					},
				},
			}

			err := c.Create(t.Context(), ra, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestRadixApplicationEnvBuildFromTypeValidation tests EnvBuild.FromType enum validation
func TestRadixApplicationEnvBuildFromTypeValidation(t *testing.T) {
	c := getClient(t)
	appName := "test-envbuild-fromtype"
	cleanup, appNamespace := createRadixRegistrationAndNamespaceForTest(t, c, appName)
	defer cleanup()

	testCases := []struct {
		name        string
		fromType    string
		shouldError bool
	}{
		{
			name:        "valid - branch",
			fromType:    "branch",
			shouldError: false,
		},
		{
			name:        "valid - tag",
			fromType:    "tag",
			shouldError: false,
		},
		{
			name:        "valid - empty",
			fromType:    "",
			shouldError: false,
		},
		{
			name:        "invalid - wrong value",
			fromType:    "commit",
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ra := &v1.RadixApplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      appName,
					Namespace: appNamespace,
				},
				Spec: v1.RadixApplicationSpec{
					Environments: []v1.Environment{
						{
							Name: "dev",
							Build: v1.EnvBuild{
								FromType: tc.fromType,
							},
						},
					},
				},
			}

			err := c.Create(t.Context(), ra, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestRadixApplicationComponentJobNameUniqueness tests CEL validation for unique component and job names
func TestRadixApplicationComponentJobNameUniqueness(t *testing.T) {
	c := getClient(t)
	appName := "test-name-uniqueness"
	cleanup, appNamespace := createRadixRegistrationAndNamespaceForTest(t, c, appName)
	defer cleanup()

	t.Run("valid - unique component and job names", func(t *testing.T) {
		ra := &v1.RadixApplication{
			ObjectMeta: metav1.ObjectMeta{
				Name:      appName,
				Namespace: appNamespace,
			},
			Spec: v1.RadixApplicationSpec{
				Environments: []v1.Environment{{Name: "dev"}},
				Components: []v1.RadixComponent{
					{Name: "frontend"},
					{Name: "backend"},
				},
				Jobs: []v1.RadixJobComponent{
					{Name: "batch-job", SchedulerPort: int32(8000)},
					{Name: "import-job", SchedulerPort: int32(8001)},
				},
			},
		}

		err := c.Create(t.Context(), ra, client.DryRunAll)
		assert.NoError(t, err)
	})

	t.Run("invalid - duplicate name in components and jobs", func(t *testing.T) {
		ra := &v1.RadixApplication{
			ObjectMeta: metav1.ObjectMeta{
				Name:      appName,
				Namespace: appNamespace,
			},
			Spec: v1.RadixApplicationSpec{
				Environments: []v1.Environment{{Name: "dev"}},
				Components: []v1.RadixComponent{
					{Name: "worker"},
					{Name: "api"},
				},
				Jobs: []v1.RadixJobComponent{
					{Name: "worker", SchedulerPort: int32(8000)}, // Duplicate with component
				},
			},
		}

		err := c.Create(t.Context(), ra, client.DryRunAll)
		assert.Error(t, err, "Should reject when component and job have the same name")
	})
}

// TestRadixApplicationComponentNameValidation tests RadixComponent.Name validation (MinLength=1, MaxLength=50, Pattern)
func TestRadixApplicationComponentNameValidation(t *testing.T) {
	c := getClient(t)
	appName := "test-component-name"
	cleanup, appNamespace := createRadixRegistrationAndNamespaceForTest(t, c, appName)
	defer cleanup()

	testCases := []struct {
		name          string
		componentName string
		shouldError   bool
	}{
		{
			name:          "valid - lowercase alphanumeric",
			componentName: "frontend",
			shouldError:   false,
		},
		{
			name:          "valid - max length (50 chars)",
			componentName: "a2345678901234567890123456789012345678901234567890",
			shouldError:   false,
		},
		{
			name:          "invalid - empty (MinLength=1)",
			componentName: "",
			shouldError:   true,
		},
		{
			name:          "invalid - exceeds max length (51 chars)",
			componentName: "a23456789012345678901234567890123456789012345678901",
			shouldError:   true,
		},
		{
			name:          "invalid - uppercase",
			componentName: "Frontend",
			shouldError:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ra := &v1.RadixApplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      appName,
					Namespace: appNamespace,
				},
				Spec: v1.RadixApplicationSpec{
					Environments: []v1.Environment{{Name: "dev"}},
					Components: []v1.RadixComponent{
						{Name: tc.componentName},
					},
				},
			}

			err := c.Create(t.Context(), ra, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestRadixApplicationComponentPortValidation tests ComponentPort validation
func TestRadixApplicationComponentPortValidation(t *testing.T) {
	c := getClient(t)
	appName := "test-port-validation"
	cleanup, appNamespace := createRadixRegistrationAndNamespaceForTest(t, c, appName)
	defer cleanup()

	testCases := []struct {
		name        string
		port        v1.ComponentPort
		shouldError bool
	}{
		{
			name: "valid - standard port",
			port: v1.ComponentPort{
				Name: "http",
				Port: 8080,
			},
			shouldError: false,
		},
		{
			name: "valid - min port (1024)",
			port: v1.ComponentPort{
				Name: "custom",
				Port: 1024,
			},
			shouldError: false,
		},
		{
			name: "valid - max port (65535)",
			port: v1.ComponentPort{
				Name: "max",
				Port: 65535,
			},
			shouldError: false,
		},
		{
			name: "invalid - port below minimum (1023)",
			port: v1.ComponentPort{
				Name: "low",
				Port: 1023,
			},
			shouldError: true,
		},
		{
			name: "invalid - port above maximum (65536)",
			port: v1.ComponentPort{
				Name: "high",
				Port: 65536,
			},
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ra := &v1.RadixApplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      appName,
					Namespace: appNamespace,
				},
				Spec: v1.RadixApplicationSpec{
					Environments: []v1.Environment{{Name: "dev"}},
					Components: []v1.RadixComponent{
						{
							Name:  "api",
							Ports: []v1.ComponentPort{tc.port},
						},
					},
				},
			}

			err := c.Create(t.Context(), ra, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestRadixApplicationEgressRuleValidation tests EgressRule validation (MinItems for destinations and ports)
func TestRadixApplicationEgressRuleValidation(t *testing.T) {
	c := getClient(t)
	appName := "test-egress-validation"
	cleanup, appNamespace := createRadixRegistrationAndNamespaceForTest(t, c, appName)
	defer cleanup()

	testCases := []struct {
		name        string
		rule        v1.EgressRule
		shouldError bool
	}{
		{
			name: "valid - single destination and port",
			rule: v1.EgressRule{
				Destinations: []v1.EgressDestination{"10.0.0.0/24"},
				Ports: []v1.EgressPort{
					{Port: 443, Protocol: "TCP"},
				},
			},
			shouldError: false,
		},
		{
			name: "valid - UDP protocol",
			rule: v1.EgressRule{
				Destinations: []v1.EgressDestination{"8.8.8.8/32"},
				Ports: []v1.EgressPort{
					{Port: 53, Protocol: "UDP"},
				},
			},
			shouldError: false,
		},
		{
			name: "invalid - empty destinations (MinItems=1)",
			rule: v1.EgressRule{
				Destinations: []v1.EgressDestination{},
				Ports: []v1.EgressPort{
					{Port: 443, Protocol: "TCP"},
				},
			},
			shouldError: true,
		},
		{
			name: "invalid - empty ports (MinItems=1)",
			rule: v1.EgressRule{
				Destinations: []v1.EgressDestination{"10.0.0.0/24"},
				Ports:        []v1.EgressPort{},
			},
			shouldError: true,
		},
		{
			name: "invalid - port below minimum (0)",
			rule: v1.EgressRule{
				Destinations: []v1.EgressDestination{"10.0.0.0/24"},
				Ports: []v1.EgressPort{
					{Port: 0, Protocol: "TCP"},
				},
			},
			shouldError: true,
		},
		{
			name: "invalid - port above maximum (65536)",
			rule: v1.EgressRule{
				Destinations: []v1.EgressDestination{"10.0.0.0/24"},
				Ports: []v1.EgressPort{
					{Port: 65536, Protocol: "TCP"},
				},
			},
			shouldError: true,
		},
		{
			name: "invalid - wrong protocol",
			rule: v1.EgressRule{
				Destinations: []v1.EgressDestination{"10.0.0.0/24"},
				Ports: []v1.EgressPort{
					{Port: 443, Protocol: "SCTP"},
				},
			},
			shouldError: true,
		},
		{
			name: "invalid - invalid CIDR format",
			rule: v1.EgressRule{
				Destinations: []v1.EgressDestination{"not-a-cidr"},
				Ports: []v1.EgressPort{
					{Port: 443, Protocol: "TCP"},
				},
			},
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ra := &v1.RadixApplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      appName,
					Namespace: appNamespace,
				},
				Spec: v1.RadixApplicationSpec{
					Environments: []v1.Environment{
						{
							Name: "dev",
							Egress: v1.EgressConfig{
								Rules: []v1.EgressRule{tc.rule},
							},
						},
					},
				},
			}

			err := c.Create(t.Context(), ra, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestRadixApplicationComponentReplicasValidation tests Component.Replicas validation (Minimum=0)
func TestRadixApplicationComponentReplicasValidation(t *testing.T) {
	c := getClient(t)
	appName := "test-replicas"
	cleanup, appNamespace := createRadixRegistrationAndNamespaceForTest(t, c, appName)
	defer cleanup()

	testCases := []struct {
		name        string
		replicas    *int
		shouldError bool
	}{
		{
			name:        "valid - nil replicas",
			replicas:    nil,
			shouldError: false,
		},
		{
			name:        "valid - zero replicas",
			replicas:    intPtr(0),
			shouldError: false,
		},
		{
			name:        "valid - multiple replicas",
			replicas:    intPtr(5),
			shouldError: false,
		},
		{
			name:        "invalid - negative replicas",
			replicas:    intPtr(-1),
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ra := &v1.RadixApplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      appName,
					Namespace: appNamespace,
				},
				Spec: v1.RadixApplicationSpec{
					Environments: []v1.Environment{{Name: "dev"}},
					Components: []v1.RadixComponent{
						{
							Name:     "app",
							Replicas: tc.replicas,
						},
					},
				},
			}

			err := c.Create(t.Context(), ra, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}
