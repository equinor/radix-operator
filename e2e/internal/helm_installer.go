package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// InstallRadixOperator installs or upgrades the radix-operator Helm chart so it runs
// with the expected image tags and configuration, whether or not it is already installed.
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

	// Install Helm chart
	cmd := exec.CommandContext(ctx, "helm", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", KubeConfigPath))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("helm upgrade/install failed: %w", err)
	}

	return nil
}

// UninstallRadixOperator uninstalls the radix-operator Helm release if it is installed. This is
// used to stop the running operator before the Radix CRDs are removed, so the operator does not
// fail when its CRDs disappear. It is a no-op if the release is not installed.
func UninstallRadixOperator(ctx context.Context, KubeConfigPath, namespace, releaseName string) error {
	env := append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", KubeConfigPath))

	// Check whether the release is installed; if not, there is nothing to uninstall
	statusCmd := exec.CommandContext(ctx, "helm", "status", releaseName, "--namespace", namespace)
	statusCmd.Env = env
	if err := statusCmd.Run(); err != nil {
		return nil
	}

	cmd := exec.CommandContext(ctx, "helm", "uninstall", releaseName, "--namespace", namespace, "--wait")
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("helm uninstall failed: %w", err)
	}

	return nil
}
