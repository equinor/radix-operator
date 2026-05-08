package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// InstallRadixOperator installs the radix-operator Helm chart
func InstallRadixOperator(ctx context.Context, KubeConfigPath, namespace, releaseName, chartPath string, values map[string]string) error {
	// Build helm upgrade --install command
	args := []string{
		"upgrade", "--install",
		releaseName,
		chartPath,
		"--namespace", namespace,
		"--create-namespace",
		"--wait",
		"--timeout", "5m",
	}

	// Add additional values
	for key, value := range values {
		args = append(args, "--set", fmt.Sprintf("%s=%v", key, value))
	}

	// Retry to handle webhook pod restarts during certificate setup
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		cmd := exec.CommandContext(ctx, "helm", args...)
		cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", KubeConfigPath))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			if attempt < maxRetries {
				fmt.Printf("helm install attempt %d/%d failed: %v. Retrying...\n", attempt, maxRetries, err)
				time.Sleep(2 * time.Second)
				continue
			}
			return fmt.Errorf("helm install failed after %d attempts: %w", maxRetries, err)
		}
		return nil
	}

	return nil
}
