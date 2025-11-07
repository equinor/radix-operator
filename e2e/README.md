# End-to-End Testing for Radix Operator

This directory contains end-to-end (e2e) integration tests for the Radix Operator. These tests create a Kind cluster, install the Helm chart, and run integration tests against it.

## Prerequisites

Before running the e2e tests, ensure you have the following tools installed:

- **Go 1.24+**: Required to run the tests
- **Kind**: Kubernetes in Docker for creating test clusters
  ```bash
  go install sigs.k8s.io/kind@latest
  ```
- **kubectl**: Kubernetes command-line tool
  ```bash
  # Install via your package manager or download from kubernetes.io
  ```
- **Helm**: Kubernetes package manager
  ```bash
  # Install via your package manager or download from helm.sh
  ```

## Running the Tests

### Se what is going on:

```bash
kind export kubeconfig --name radix-operator-e2e
kubectl get pods -Aw
```

### Run all e2e tests

```bash
cd e2e
go test -v -timeout 30m ./...
```

### Run a specific test

```bash
cd e2e
go test -v -timeout 30m -run TestRadixRegistrationWebhook
```

### Run with race detection

```bash
cd e2e
go test -v -race -timeout 30m ./...
```

## Test Structure

The e2e test suite consists of several components:

### Test Files (Main Package)

- **`setup_test.go`**: Test suite setup and teardown using TestMain
- **`radix_registration_test.go`**: Tests for RadixRegistration CRD validation and CRUD operations
- **`example_test.go`**: Example test demonstrating best practices

### Internal Package (`internal/`)

Infrastructure and helper code used by tests:

- **`kind_cluster.go`**: Kind cluster lifecycle management
- **`helm_installer.go`**: Helm chart installation
- **`prometheus_installer.go`**: Prometheus Operator CRD setup
- **`clients.go`**: Kubernetes and Radix client initialization
- **`test_helpers.go`**: Utility functions for test assertions and resource management

## Test Flow

1. **Setup** (`TestMain`):
   - Creates a Kind cluster named `radix-operator-e2e`
   - Generates a temporary kubeconfig
   - Installs Prometheus Operator CRDs (required for ServiceMonitor resources)
   - Installs the radix-operator Helm chart with test configuration
   - Initializes Kubernetes and Radix clients

2. **Test Execution**:
   - Each test runs against the shared Kind cluster
   - Tests validate webhook behavior, CRUD operations, and resource state

3. **Teardown** (`TestMain`):
   - Deletes the Kind cluster
   - Cleans up temporary files

## Configuration

### Prometheus Operator CRDs

The test suite automatically installs the Prometheus Operator CRDs from GitHub. The version is dynamically determined from the `go.mod` dependencies at runtime by inspecting the build information. This ensures the CRD version matches the Prometheus Operator version used by radix-operator.

The CRDs are downloaded from:
```
https://github.com/prometheus-operator/prometheus-operator/releases/download/{version}/stripped-down-crds.yaml
```

This is required because the radix-operator uses ServiceMonitor resources.

### Helm Chart Configuration

The Helm chart is installed with the following configuration:

```yaml
rbac:
  createApp:
    groups:
      - "123"
```

This is set via: `--set "rbac.createApp.groups[0]=123"`

## Writing New Tests

To add new e2e tests:

1. Create a new test file in the `e2e` directory (e.g., `my_feature_test.go`)
2. Import the internal package: `"github.com/equinor/radix-operator/e2e/internal"`
3. Use helper functions from `internal.NewTestHelpers()`
4. Access the manager via `getManager(t)`, client via `getClient(t)`, or config via `getKubeConfig(t)`
5. Use the test context from `getTestContext(t)`

Example:

```go
package e2e

import (
    "testing"
    "github.com/equinor/radix-operator/e2e/internal"
    "github.com/stretchr/testify/require"
)

func TestMyFeature(t *testing.T) {
    ctx := getTestContext(t)
    client := getClient(t)
    config := getKubeConfig(t)
    
    clients, err := internal.NewClients(client, config)
    require.NoError(t, err)
    
    helpers := internal.NewTestHelpers(clients)
    
    // Your test logic here
}
```

## Troubleshooting

### Get kubecontext:

```bash
kind export kubeconfig --name radix-operator-e2e
kubectl get deployments -Aw
```

### Tests timeout

Increase the timeout when running tests:
```bash
go test -v -timeout 60m ./...
```

### Kind cluster already exists

Delete the existing cluster manually:
```bash
kind delete cluster --name radix-operator-e2e
```

### CRDs not installed

The Helm chart should install CRDs automatically. If tests fail due to missing CRDs, verify:
```bash
kubectl --kubeconfig /tmp/kind-kubeconfig-*/kubeconfig get crds
```

## Cleanup

The test suite automatically cleans up the Kind cluster after tests complete. If cleanup fails or tests are interrupted, manually delete the cluster:

```bash
kind delete cluster --name radix-operator-e2e
```

## CI/CD Integration

To integrate these tests into CI/CD pipelines:

1. Ensure all prerequisites are installed in the CI environment
2. Run tests with appropriate timeout: `go test -v -timeout 30m ./e2e/...`
3. Collect test results and logs
4. Clean up resources even on failure

Example for GitHub Actions:

```yaml
- name: Run e2e tests
  run: |
    cd e2e
    go test -v -timeout 30m ./...
  timeout-minutes: 35
```
