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

### Core Files

- **`setup_test.go`**: Test suite setup and teardown, including cluster creation and Helm installation
- **`kind_cluster.go`**: Kind cluster management utilities
- **`helm_installer.go`**: Helm chart installation and management
- **`clients.go`**: Kubernetes and Radix client setup
- **`test_helpers.go`**: Utility functions for test assertions and resource management

### Test Files

- **`radix_registration_test.go`**: Tests for RadixRegistration CRD validation and CRUD operations

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
2. Use the helper functions from `test_helpers.go`
3. Access clients via `getKubeClient(t)`, `getKubeConfig(t)`, or create new clients with `NewClients()`
4. Use the test context from `getTestContext(t)`

Example:

```go
package e2e

import (
    "testing"
    "github.com/stretchr/testify/require"
)

func TestMyFeature(t *testing.T) {
    ctx := getTestContext(t)
    config := getKubeConfig(t)
    
    clients, err := NewClients(config)
    require.NoError(t, err)
    
    helpers := NewTestHelpers(clients)
    
    // Your test logic here
}
```

## Troubleshooting

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

### Webhook not responding

Check if the webhook pod is running:
```bash
kubectl --kubeconfig /tmp/kind-kubeconfig-*/kubeconfig get pods -A
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
