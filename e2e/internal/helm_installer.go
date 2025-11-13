package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// InstallRadixOperator installs the radix-operator Helm chart
func InstallRadixOperator(ctx context.Context, KubeConfigPath, namespace, releaseName, chartPath string, values map[string]string) error {
	// Build helm install command
	args := []string{
		"install",
		releaseName,
		chartPath,
		"--namespace", namespace,
		"--create-namespace",
		"--wait",
		"--timeout", "5m",
	}

	// Add additional values
	for key, value := range values {
		key = strings.TrimPrefix(key, ".")
		args = append(args, "--set", fmt.Sprintf("%s=%v", key, value))
	}

	// Install Helm chart
	cmd := exec.CommandContext(ctx, "helm", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", KubeConfigPath))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("helm install failed: %w", err)
	}

	return nil
}
