package e2e

import (
	"strings"
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestRadixDeploymentSpecAppNameValidation tests AppName validation rules
func TestRadixDeploymentSpecAppNameValidation(t *testing.T) {
	c := getClient(t)
	testCases := []struct {
		name        string
		appName     string
		shouldError bool
		description string
	}{
		{
			name:        "valid - lowercase alphanumeric",
			appName:     "myapp",
			shouldError: false,
			description: "simple lowercase name",
		},
		{
			name:        "valid - with hyphens",
			appName:     "my-app-123",
			shouldError: false,
			description: "lowercase with hyphens and numbers",
		},
		{
			name:        "valid - starts and ends with alphanumeric",
			appName:     "a-b-c",
			shouldError: false,
			description: "single chars separated by hyphens",
		},
		{
			name:        "valid - max length 253",
			appName:     strings.Repeat("a", 253),
			shouldError: false,
			description: "253 characters (max length)",
		},
		{
			name:        "invalid - empty string",
			appName:     "",
			shouldError: true,
			description: "empty appName violates MinLength=1",
		},
		{
			name:        "invalid - uppercase",
			appName:     "MyApp",
			shouldError: true,
			description: "uppercase letters not allowed",
		},
		{
			name:        "invalid - starts with hyphen",
			appName:     "-myapp",
			shouldError: true,
			description: "cannot start with hyphen",
		},
		{
			name:        "invalid - ends with hyphen",
			appName:     "myapp-",
			shouldError: true,
			description: "cannot end with hyphen",
		},
		{
			name:        "invalid - underscore",
			appName:     "my_app",
			shouldError: true,
			description: "underscore not allowed",
		},
		{
			name:        "invalid - exceeds max length",
			appName:     strings.Repeat("a", 254),
			shouldError: true,
			description: "254 characters exceeds MaxLength=253",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rd := &v1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rd",
					Namespace: "default",
				},
				Spec: v1.RadixDeploymentSpec{
					AppName:     tc.appName,
					Environment: "dev",
				},
			}

			err := c.Create(t.Context(), rd, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err, "Expected validation error for appName: %s (%s)", tc.appName, tc.description)
				if err != nil {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				assert.NoError(t, err, "Should accept valid appName: %s (%s)", tc.appName, tc.description)
			}
		})
	}
}

// TestRadixDeploymentSpecEnvironmentValidation tests Environment validation rules
func TestRadixDeploymentSpecEnvironmentValidation(t *testing.T) {
	c := getClient(t)
	testCases := []struct {
		name        string
		environment string
		shouldError bool
		description string
	}{
		{
			name:        "valid - dev",
			environment: "dev",
			shouldError: false,
			description: "simple environment name",
		},
		{
			name:        "valid - with hyphens",
			environment: "qa-environment",
			shouldError: false,
			description: "environment with hyphens",
		},
		{
			name:        "valid - max length 63",
			environment: strings.Repeat("a", 63),
			shouldError: false,
			description: "63 characters (max length)",
		},
		{
			name:        "invalid - empty string",
			environment: "",
			shouldError: true,
			description: "empty environment violates MinLength=1",
		},
		{
			name:        "invalid - uppercase",
			environment: "Production",
			shouldError: true,
			description: "uppercase letters not allowed",
		},
		{
			name:        "invalid - starts with hyphen",
			environment: "-dev",
			shouldError: true,
			description: "cannot start with hyphen",
		},
		{
			name:        "invalid - ends with hyphen",
			environment: "dev-",
			shouldError: true,
			description: "cannot end with hyphen",
		},
		{
			name:        "invalid - exceeds max length",
			environment: strings.Repeat("a", 64),
			shouldError: true,
			description: "64 characters exceeds MaxLength=63",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rd := &v1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rd",
					Namespace: "default",
				},
				Spec: v1.RadixDeploymentSpec{
					AppName:     "myapp",
					Environment: tc.environment,
				},
			}

			err := c.Create(t.Context(), rd, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err, "Expected validation error for environment: %s (%s)", tc.environment, tc.description)
				if err != nil {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				assert.NoError(t, err, "Should accept valid environment: %s (%s)", tc.environment, tc.description)
			}
		})
	}
}

// TestRadixDeployComponentNameValidation tests component Name validation rules
func TestRadixDeployComponentNameValidation(t *testing.T) {
	c := getClient(t)
	testCases := []struct {
		name          string
		componentName string
		shouldError   bool
		description   string
	}{
		{
			name:          "valid - simple name",
			componentName: "web",
			shouldError:   false,
			description:   "simple component name",
		},
		{
			name:          "valid - with hyphens",
			componentName: "web-frontend",
			shouldError:   false,
			description:   "component with hyphens",
		},
		{
			name:          "valid - max length 50",
			componentName: strings.Repeat("a", 50),
			shouldError:   false,
			description:   "50 characters (max length)",
		},
		{
			name:          "invalid - empty string",
			componentName: "",
			shouldError:   true,
			description:   "empty name violates MinLength=1",
		},
		{
			name:          "invalid - uppercase",
			componentName: "WebFrontend",
			shouldError:   true,
			description:   "uppercase letters not allowed",
		},
		{
			name:          "invalid - starts with hyphen",
			componentName: "-web",
			shouldError:   true,
			description:   "cannot start with hyphen",
		},
		{
			name:          "invalid - ends with hyphen",
			componentName: "web-",
			shouldError:   true,
			description:   "cannot end with hyphen",
		},
		{
			name:          "invalid - exceeds max length",
			componentName: strings.Repeat("a", 51),
			shouldError:   true,
			description:   "51 characters exceeds MaxLength=50",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rd := &v1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rd",
					Namespace: "default",
				},
				Spec: v1.RadixDeploymentSpec{
					AppName:     "myapp",
					Environment: "dev",
					Components: []v1.RadixDeployComponent{
						{
							Name:  tc.componentName,
							Image: "myimage:latest",
						},
					},
				},
			}

			err := c.Create(t.Context(), rd, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err, "Expected validation error for component name: %s (%s)", tc.componentName, tc.description)
				if err != nil {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				assert.NoError(t, err, "Should accept valid component name: %s (%s)", tc.componentName, tc.description)
			}
		})
	}
}

// TestRadixDeployComponentImageValidation tests component Image validation rules
func TestRadixDeployComponentImageValidation(t *testing.T) {
	c := getClient(t)
	testCases := []struct {
		name        string
		image       string
		shouldError bool
		description string
	}{
		{
			name:        "valid - simple image",
			image:       "nginx",
			shouldError: false,
			description: "simple image name",
		},
		{
			name:        "valid - with tag",
			image:       "nginx:1.21",
			shouldError: false,
			description: "image with tag",
		},
		{
			name:        "valid - with registry",
			image:       "docker.io/library/nginx:latest",
			shouldError: false,
			description: "full image path with registry",
		},
		{
			name:        "invalid - empty string",
			image:       "",
			shouldError: true,
			description: "empty image violates MinLength=1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rd := &v1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rd",
					Namespace: "default",
				},
				Spec: v1.RadixDeploymentSpec{
					AppName:     "myapp",
					Environment: "dev",
					Components: []v1.RadixDeployComponent{
						{
							Name:  "web",
							Image: tc.image,
						},
					},
				},
			}

			err := c.Create(t.Context(), rd, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err, "Expected validation error for image: %s (%s)", tc.image, tc.description)
				if err != nil {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				assert.NoError(t, err, "Should accept valid image: %s (%s)", tc.image, tc.description)
			}
		})
	}
}

// TestRadixDeployComponentReplicasValidation tests component Replicas validation rules
func TestRadixDeployComponentReplicasValidation(t *testing.T) {
	c := getClient(t)
	intPtr := func(i int) *int { return &i }

	testCases := []struct {
		name        string
		replicas    *int
		shouldError bool
		description string
	}{
		{
			name:        "valid - 0 replicas",
			replicas:    intPtr(0),
			shouldError: false,
			description: "0 replicas (minimum)",
		},
		{
			name:        "valid - 1 replica",
			replicas:    intPtr(1),
			shouldError: false,
			description: "1 replica",
		},
		{
			name:        "valid - 64 replicas",
			replicas:    intPtr(64),
			shouldError: false,
			description: "64 replicas (maximum)",
		},
		{
			name:        "valid - nil replicas",
			replicas:    nil,
			shouldError: false,
			description: "nil replicas (optional field)",
		},
		{
			name:        "invalid - negative replicas",
			replicas:    intPtr(-1),
			shouldError: true,
			description: "negative replicas violates Minimum=0",
		},
		{
			name:        "invalid - exceeds maximum",
			replicas:    intPtr(65),
			shouldError: true,
			description: "65 replicas exceeds Maximum=64",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rd := &v1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rd",
					Namespace: "default",
				},
				Spec: v1.RadixDeploymentSpec{
					AppName:     "myapp",
					Environment: "dev",
					Components: []v1.RadixDeployComponent{
						{
							Name:     "web",
							Image:    "nginx:latest",
							Replicas: tc.replicas,
						},
					},
				},
			}

			err := c.Create(t.Context(), rd, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err, "Expected validation error for replicas: %v (%s)", tc.replicas, tc.description)
				if err != nil {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				assert.NoError(t, err, "Should accept valid replicas: %v (%s)", tc.replicas, tc.description)
			}
		})
	}
}

// TestRadixDeployComponentReplicasOverrideValidation tests component ReplicasOverride validation rules
func TestRadixDeployComponentReplicasOverrideValidation(t *testing.T) {
	c := getClient(t)
	intPtr := func(i int) *int { return &i }

	testCases := []struct {
		name             string
		replicasOverride *int
		shouldError      bool
		description      string
	}{
		{
			name:             "valid - 0 replicas",
			replicasOverride: intPtr(0),
			shouldError:      false,
			description:      "0 replicas override (minimum)",
		},
		{
			name:             "valid - 10 replicas",
			replicasOverride: intPtr(10),
			shouldError:      false,
			description:      "10 replicas override",
		},
		{
			name:             "valid - 64 replicas",
			replicasOverride: intPtr(64),
			shouldError:      false,
			description:      "64 replicas override (maximum)",
		},
		{
			name:             "valid - nil",
			replicasOverride: nil,
			shouldError:      false,
			description:      "nil replicas override (optional field)",
		},
		{
			name:             "invalid - negative",
			replicasOverride: intPtr(-1),
			shouldError:      true,
			description:      "negative replicas override violates Minimum=0",
		},
		{
			name:             "invalid - exceeds maximum",
			replicasOverride: intPtr(65),
			shouldError:      true,
			description:      "65 replicas override exceeds Maximum=64",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rd := &v1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rd",
					Namespace: "default",
				},
				Spec: v1.RadixDeploymentSpec{
					AppName:     "myapp",
					Environment: "dev",
					Components: []v1.RadixDeployComponent{
						{
							Name:             "web",
							Image:            "nginx:latest",
							ReplicasOverride: tc.replicasOverride,
						},
					},
				},
			}

			err := c.Create(t.Context(), rd, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err, "Expected validation error for replicasOverride: %v (%s)", tc.replicasOverride, tc.description)
				if err != nil {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				assert.NoError(t, err, "Should accept valid replicasOverride: %v (%s)", tc.replicasOverride, tc.description)
			}
		})
	}
}

// TestRadixDeployComponentRunAsUserValidation tests component RunAsUser validation rules
func TestRadixDeployComponentRunAsUserValidation(t *testing.T) {
	c := getClient(t)
	int64Ptr := func(i int64) *int64 { return &i }

	testCases := []struct {
		name        string
		runAsUser   *int64
		shouldError bool
		description string
	}{
		{
			name:        "valid - user 1000",
			runAsUser:   int64Ptr(1000),
			shouldError: false,
			description: "user ID 1000",
		},
		{
			name:        "valid - user 1 (minimum)",
			runAsUser:   int64Ptr(1),
			shouldError: false,
			description: "user ID 1 (minimum)",
		},
		{
			name:        "valid - nil",
			runAsUser:   nil,
			shouldError: false,
			description: "nil runAsUser (optional field)",
		},
		{
			name:        "invalid - user 0",
			runAsUser:   int64Ptr(0),
			shouldError: true,
			description: "user ID 0 violates Minimum=1",
		},
		{
			name:        "invalid - negative user",
			runAsUser:   int64Ptr(-1),
			shouldError: true,
			description: "negative user ID violates Minimum=1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rd := &v1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rd",
					Namespace: "default",
				},
				Spec: v1.RadixDeploymentSpec{
					AppName:     "myapp",
					Environment: "dev",
					Components: []v1.RadixDeployComponent{
						{
							Name:      "web",
							Image:     "nginx:latest",
							RunAsUser: tc.runAsUser,
						},
					},
				},
			}

			err := c.Create(t.Context(), rd, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err, "Expected validation error for runAsUser: %v (%s)", tc.runAsUser, tc.description)
				if err != nil {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				assert.NoError(t, err, "Should accept valid runAsUser: %v (%s)", tc.runAsUser, tc.description)
			}
		})
	}
}

// TestRadixDeployExternalDNSValidation tests ExternalDNS FQDN validation rules
func TestRadixDeployExternalDNSValidation(t *testing.T) {
	c := getClient(t)
	testCases := []struct {
		name        string
		fqdn        string
		shouldError bool
		description string
	}{
		{
			name:        "valid - simple domain",
			fqdn:        "example.com",
			shouldError: false,
			description: "simple FQDN",
		},
		{
			name:        "valid - subdomain",
			fqdn:        "app.example.com",
			shouldError: false,
			description: "FQDN with subdomain",
		},
		{
			name:        "valid - multiple subdomains",
			fqdn:        "my.app.example.com",
			shouldError: false,
			description: "FQDN with multiple subdomains",
		},
		{
			name:        "valid - with hyphens",
			fqdn:        "my-app.example-domain.com",
			shouldError: false,
			description: "FQDN with hyphens",
		},
		{
			name:        "valid - min length 4",
			fqdn:        "a.bc",
			shouldError: false,
			description: "4 characters (minimum length)",
		},
		{
			name:        "valid - max length 255",
			fqdn:        strings.Repeat("a", 243) + ".example.com",
			shouldError: false,
			description: "255 characters (maximum length)",
		},
		{
			name:        "invalid - too short",
			fqdn:        "abc",
			shouldError: true,
			description: "3 characters violates MinLength=4",
		},
		{
			name:        "invalid - empty",
			fqdn:        "",
			shouldError: true,
			description: "empty FQDN violates MinLength=4",
		},
		{
			name:        "invalid - exceeds max length",
			fqdn:        strings.Repeat("a", 256),
			shouldError: true,
			description: "256 characters exceeds MaxLength=255",
		},
		{
			name:        "invalid - starts with hyphen",
			fqdn:        "-app.example.com",
			shouldError: true,
			description: "label cannot start with hyphen",
		},
		{
			name:        "invalid - ends with hyphen",
			fqdn:        "app-.example.com",
			shouldError: true,
			description: "label cannot end with hyphen",
		},
		{
			name:        "invalid - double dots",
			fqdn:        "app..example.com",
			shouldError: true,
			description: "double dots not allowed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rd := &v1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rd",
					Namespace: "default",
				},
				Spec: v1.RadixDeploymentSpec{
					AppName:     "myapp",
					Environment: "dev",
					Components: []v1.RadixDeployComponent{
						{
							Name:  "web",
							Image: "nginx:latest",
							ExternalDNS: []v1.RadixDeployExternalDNS{
								{
									FQDN:                     tc.fqdn,
									UseCertificateAutomation: false,
								},
							},
						},
					},
				},
			}

			err := c.Create(t.Context(), rd, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err, "Expected validation error for FQDN: %s (%s)", tc.fqdn, tc.description)
				if err != nil {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				assert.NoError(t, err, "Should accept valid FQDN: %s (%s)", tc.fqdn, tc.description)
			}
		})
	}
}

// TestRadixDeployJobComponentNameValidation tests job component Name validation rules
func TestRadixDeployJobComponentNameValidation(t *testing.T) {
	c := getClient(t)
	testCases := []struct {
		name        string
		jobName     string
		shouldError bool
		description string
	}{
		{
			name:        "valid - simple name",
			jobName:     "compute",
			shouldError: false,
			description: "simple job name",
		},
		{
			name:        "valid - with hyphens",
			jobName:     "batch-processor",
			shouldError: false,
			description: "job with hyphens",
		},
		{
			name:        "valid - max length 253",
			jobName:     strings.Repeat("a", 253),
			shouldError: false,
			description: "253 characters (max length)",
		},
		{
			name:        "invalid - empty string",
			jobName:     "",
			shouldError: true,
			description: "empty name violates MinLength=1",
		},
		{
			name:        "invalid - uppercase",
			jobName:     "BatchJob",
			shouldError: true,
			description: "uppercase letters not allowed",
		},
		{
			name:        "invalid - starts with hyphen",
			jobName:     "-compute",
			shouldError: true,
			description: "cannot start with hyphen",
		},
		{
			name:        "invalid - ends with hyphen",
			jobName:     "compute-",
			shouldError: true,
			description: "cannot end with hyphen",
		},
		{
			name:        "invalid - exceeds max length",
			jobName:     strings.Repeat("a", 254),
			shouldError: true,
			description: "254 characters exceeds MaxLength=253",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rd := &v1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rd",
					Namespace: "default",
				},
				Spec: v1.RadixDeploymentSpec{
					AppName:     "myapp",
					Environment: "dev",
					Jobs: []v1.RadixDeployJobComponent{
						{
							Name:          tc.jobName,
							Image:         "myimage:latest",
							SchedulerPort: 8080,
						},
					},
				},
			}

			err := c.Create(t.Context(), rd, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err, "Expected validation error for job name: %s (%s)", tc.jobName, tc.description)
				if err != nil {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				assert.NoError(t, err, "Should accept valid job name: %s (%s)", tc.jobName, tc.description)
			}
		})
	}
}

// TestRadixDeployJobComponentImageValidation tests job component Image validation rules
func TestRadixDeployJobComponentImageValidation(t *testing.T) {
	c := getClient(t)
	testCases := []struct {
		name        string
		image       string
		shouldError bool
		description string
	}{
		{
			name:        "valid - simple image",
			image:       "python",
			shouldError: false,
			description: "simple image name",
		},
		{
			name:        "valid - with tag",
			image:       "python:3.9",
			shouldError: false,
			description: "image with tag",
		},
		{
			name:        "invalid - empty string",
			image:       "",
			shouldError: true,
			description: "empty image violates MinLength=1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rd := &v1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rd",
					Namespace: "default",
				},
				Spec: v1.RadixDeploymentSpec{
					AppName:     "myapp",
					Environment: "dev",
					Jobs: []v1.RadixDeployJobComponent{
						{
							Name:          "compute",
							Image:         tc.image,
							SchedulerPort: 8080,
						},
					},
				},
			}

			err := c.Create(t.Context(), rd, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err, "Expected validation error for job image: %s (%s)", tc.image, tc.description)
				if err != nil {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				assert.NoError(t, err, "Should accept valid job image: %s (%s)", tc.image, tc.description)
			}
		})
	}
}

// TestRadixDeployJobComponentSchedulerPortValidation tests job component SchedulerPort validation rules
func TestRadixDeployJobComponentSchedulerPortValidation(t *testing.T) {
	c := getClient(t)
	testCases := []struct {
		name          string
		schedulerPort int32
		shouldError   bool
		description   string
	}{
		{
			name:          "valid - port 1024 (minimum)",
			schedulerPort: 1024,
			shouldError:   false,
			description:   "1024 (minimum port)",
		},
		{
			name:          "valid - port 8080",
			schedulerPort: 8080,
			shouldError:   false,
			description:   "common port 8080",
		},
		{
			name:          "valid - port 65535 (maximum)",
			schedulerPort: 65535,
			shouldError:   false,
			description:   "65535 (maximum port)",
		},
		{
			name:          "invalid - port 1023",
			schedulerPort: 1023,
			shouldError:   true,
			description:   "1023 violates Minimum=1024",
		},
		{
			name:          "invalid - port 0",
			schedulerPort: 0,
			shouldError:   true,
			description:   "0 violates Minimum=1024",
		},
		{
			name:          "invalid - port 65536",
			schedulerPort: 65536,
			shouldError:   true,
			description:   "65536 exceeds Maximum=65535",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rd := &v1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rd",
					Namespace: "default",
				},
				Spec: v1.RadixDeploymentSpec{
					AppName:     "myapp",
					Environment: "dev",
					Jobs: []v1.RadixDeployJobComponent{
						{
							Name:          "compute",
							Image:         "python:3.9",
							SchedulerPort: tc.schedulerPort,
						},
					},
				},
			}

			err := c.Create(t.Context(), rd, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err, "Expected validation error for schedulerPort: %d (%s)", tc.schedulerPort, tc.description)
				if err != nil {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				assert.NoError(t, err, "Should accept valid schedulerPort: %d (%s)", tc.schedulerPort, tc.description)
			}
		})
	}
}

// TestRadixDeployJobComponentTimeLimitSecondsValidation tests job component TimeLimitSeconds validation rules
func TestRadixDeployJobComponentTimeLimitSecondsValidation(t *testing.T) {
	c := getClient(t)
	int64Ptr := func(i int64) *int64 { return &i }

	testCases := []struct {
		name             string
		timeLimitSeconds *int64
		shouldError      bool
		description      string
	}{
		{
			name:             "valid - 1 second (minimum)",
			timeLimitSeconds: int64Ptr(1),
			shouldError:      false,
			description:      "1 second (minimum)",
		},
		{
			name:             "valid - 3600 seconds",
			timeLimitSeconds: int64Ptr(3600),
			shouldError:      false,
			description:      "3600 seconds (1 hour)",
		},
		{
			name:             "valid - nil",
			timeLimitSeconds: nil,
			shouldError:      false,
			description:      "nil timeLimitSeconds (optional field)",
		},
		{
			name:             "invalid - 0 seconds",
			timeLimitSeconds: int64Ptr(0),
			shouldError:      true,
			description:      "0 seconds violates Minimum=1",
		},
		{
			name:             "invalid - negative",
			timeLimitSeconds: int64Ptr(-1),
			shouldError:      true,
			description:      "negative seconds violates Minimum=1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rd := &v1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rd",
					Namespace: "default",
				},
				Spec: v1.RadixDeploymentSpec{
					AppName:     "myapp",
					Environment: "dev",
					Jobs: []v1.RadixDeployJobComponent{
						{
							Name:             "compute",
							Image:            "python:3.9",
							SchedulerPort:    8080,
							TimeLimitSeconds: tc.timeLimitSeconds,
						},
					},
				},
			}

			err := c.Create(t.Context(), rd, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err, "Expected validation error for timeLimitSeconds: %v (%s)", tc.timeLimitSeconds, tc.description)
				if err != nil {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				assert.NoError(t, err, "Should accept valid timeLimitSeconds: %v (%s)", tc.timeLimitSeconds, tc.description)
			}
		})
	}
}

// TestRadixDeployJobComponentBackoffLimitValidation tests job component BackoffLimit validation rules
func TestRadixDeployJobComponentBackoffLimitValidation(t *testing.T) {
	c := getClient(t)
	int32Ptr := func(i int32) *int32 { return &i }

	testCases := []struct {
		name         string
		backoffLimit *int32
		shouldError  bool
		description  string
	}{
		{
			name:         "valid - 0 (minimum)",
			backoffLimit: int32Ptr(0),
			shouldError:  false,
			description:  "0 (minimum)",
		},
		{
			name:         "valid - 6",
			backoffLimit: int32Ptr(6),
			shouldError:  false,
			description:  "6 retries",
		},
		{
			name:         "valid - nil",
			backoffLimit: nil,
			shouldError:  false,
			description:  "nil backoffLimit (optional field)",
		},
		{
			name:         "invalid - negative",
			backoffLimit: int32Ptr(-1),
			shouldError:  true,
			description:  "negative value violates Minimum=0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rd := &v1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rd",
					Namespace: "default",
				},
				Spec: v1.RadixDeploymentSpec{
					AppName:     "myapp",
					Environment: "dev",
					Jobs: []v1.RadixDeployJobComponent{
						{
							Name:          "compute",
							Image:         "python:3.9",
							SchedulerPort: 8080,
							BackoffLimit:  tc.backoffLimit,
						},
					},
				},
			}

			err := c.Create(t.Context(), rd, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err, "Expected validation error for backoffLimit: %v (%s)", tc.backoffLimit, tc.description)
				if err != nil {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				assert.NoError(t, err, "Should accept valid backoffLimit: %v (%s)", tc.backoffLimit, tc.description)
			}
		})
	}
}

// TestRadixDeployJobComponentRunAsUserValidation tests job component RunAsUser validation rules
func TestRadixDeployJobComponentRunAsUserValidation(t *testing.T) {
	c := getClient(t)
	int64Ptr := func(i int64) *int64 { return &i }

	testCases := []struct {
		name        string
		runAsUser   *int64
		shouldError bool
		description string
	}{
		{
			name:        "valid - user 1000",
			runAsUser:   int64Ptr(1000),
			shouldError: false,
			description: "user ID 1000",
		},
		{
			name:        "valid - user 1 (minimum)",
			runAsUser:   int64Ptr(1),
			shouldError: false,
			description: "user ID 1 (minimum)",
		},
		{
			name:        "valid - nil",
			runAsUser:   nil,
			shouldError: false,
			description: "nil runAsUser (optional field)",
		},
		{
			name:        "invalid - user 0",
			runAsUser:   int64Ptr(0),
			shouldError: true,
			description: "user ID 0 violates Minimum=1",
		},
		{
			name:        "invalid - negative user",
			runAsUser:   int64Ptr(-1),
			shouldError: true,
			description: "negative user ID violates Minimum=1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rd := &v1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rd",
					Namespace: "default",
				},
				Spec: v1.RadixDeploymentSpec{
					AppName:     "myapp",
					Environment: "dev",
					Jobs: []v1.RadixDeployJobComponent{
						{
							Name:          "compute",
							Image:         "python:3.9",
							SchedulerPort: 8080,
							RunAsUser:     tc.runAsUser,
						},
					},
				},
			}

			err := c.Create(t.Context(), rd, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err, "Expected validation error for job runAsUser: %v (%s)", tc.runAsUser, tc.description)
				if err != nil {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				assert.NoError(t, err, "Should accept valid job runAsUser: %v (%s)", tc.runAsUser, tc.description)
			}
		})
	}
}

// TestRadixDeploymentCompleteSpec tests a complete valid RadixDeployment
func TestRadixDeploymentCompleteSpec(t *testing.T) {
	c := getClient(t)
	int64Ptr := func(i int64) *int64 { return &i }
	int32Ptr := func(i int32) *int32 { return &i }
	intPtr := func(i int) *int { return &i }

	rd := &v1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-complete-rd",
			Namespace: "default",
		},
		Spec: v1.RadixDeploymentSpec{
			AppName:     "myapp",
			Environment: "production",
			Components: []v1.RadixDeployComponent{
				{
					Name:             "frontend",
					Image:            "myregistry/frontend:v1.2.3",
					Replicas:         intPtr(3),
					ReplicasOverride: intPtr(5),
					RunAsUser:        int64Ptr(1000),
					PublicPort:       "8080",
					ExternalDNS: []v1.RadixDeployExternalDNS{
						{
							FQDN:                     "myapp.example.com",
							UseCertificateAutomation: true,
						},
					},
				},
				{
					Name:      "backend",
					Image:     "myregistry/backend:v1.2.3",
					Replicas:  intPtr(2),
					RunAsUser: int64Ptr(1001),
				},
			},
			Jobs: []v1.RadixDeployJobComponent{
				{
					Name:             "data-processor",
					Image:            "myregistry/processor:v1.0.0",
					SchedulerPort:    8080,
					TimeLimitSeconds: int64Ptr(3600),
					BackoffLimit:     int32Ptr(3),
					RunAsUser:        int64Ptr(1002),
				},
			},
			ImagePullSecrets: []corev1.LocalObjectReference{
				{Name: "my-registry-secret"},
			},
		},
	}

	err := c.Create(t.Context(), rd, client.DryRunAll)
	require.NoError(t, err, "Should accept valid complete RadixDeployment")
}

// TestRadixDeploymentMinimalSpec tests a minimal valid RadixDeployment
func TestRadixDeploymentMinimalSpec(t *testing.T) {
	c := getClient(t)

	rd := &v1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-minimal-rd",
			Namespace: "default",
		},
		Spec: v1.RadixDeploymentSpec{
			AppName:     "a",
			Environment: "d",
		},
	}

	err := c.Create(t.Context(), rd, client.DryRunAll)
	require.NoError(t, err, "Should accept minimal valid RadixDeployment")
}
