package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"time"
)

// HelmInstaller handles Helm chart installation
type HelmInstaller struct {
	KubeConfigPath string
}

// HelmInstallConfig holds configuration for Helm installation
type HelmInstallConfig struct {
	ChartPath   string
	ReleaseName string
	Namespace   string
	Values      map[string]string
}

// NewHelmInstaller creates a new Helm installer
func NewHelmInstaller(kubeConfigPath string) *HelmInstaller {
	return &HelmInstaller{
		KubeConfigPath: kubeConfigPath,
	}
}

// InstallRadixOperator installs the radix-operator Helm chart
func (h *HelmInstaller) InstallRadixOperator(ctx context.Context, config HelmInstallConfig) error {
	// Build helm command
	args := []string{
		"template",
		config.ReleaseName,
		config.ChartPath,
	}

	// Add additional values
	for key, value := range config.Values {
		args = append(args, "--set", fmt.Sprintf("%s=%v", key, value))
	}

	// Generate Helm template
	cmd := exec.CommandContext(ctx, "helm", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", h.KubeConfigPath))

	manifestBytes, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("helm template failed: %w, stderr: %s", err, string(exitErr.Stderr))
		}
		return fmt.Errorf("helm template failed: %w", err)
	}

	// Apply manifests using kubectl
	applyCmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	applyCmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", h.KubeConfigPath))
	applyCmd.Stdin = strings.NewReader(string(manifestBytes))
	applyCmd.Stdout = os.Stdout
	applyCmd.Stderr = os.Stderr

	if err := applyCmd.Run(); err != nil {
		return fmt.Errorf("kubectl apply failed: %w", err)
	}

	// Wait for deployment to be ready
	if err := h.waitForDeployment(ctx, "radix-operator", config.Namespace); err != nil {
		return fmt.Errorf("deployment not ready: %w", err)
	}

	return nil
}

// waitForDeployment waits for a deployment to be ready
func (h *HelmInstaller) waitForDeployment(ctx context.Context, name, namespace string) error {
	timeout := 5 * time.Minute
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		args := []string{
			"--kubeconfig", h.KubeConfigPath,
			"get", "deployment",
			name,
			"-n", namespace,
		}

		cmd := exec.CommandContext(ctx, "kubectl", args...)
		if err := cmd.Run(); err == nil {
			// Check if deployment is ready
			rolloutArgs := []string{
				"--kubeconfig", h.KubeConfigPath,
				"rollout", "status",
				"deployment/" + name,
				"-n", namespace,
				"--timeout=10s",
			}
			rolloutCmd := exec.CommandContext(ctx, "kubectl", rolloutArgs...)
			if err := rolloutCmd.Run(); err == nil {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			continue
		}
	}

	return fmt.Errorf("deployment %s not ready after %v", name, timeout)
}

// getPrometheusOperatorVersion gets the version from build info
func getPrometheusOperatorVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		// Fallback to a default version if build info is not available
		return "v0.76.0"
	}

	for _, dep := range info.Deps {
		if dep.Path == "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring" {
			return dep.Version
		}
	}

	// Fallback to a default version
	return "v0.76.0"
}

// InstallPrometheusOperatorCRDs installs the Prometheus Operator CRDs from GitHub
func (h *HelmInstaller) InstallPrometheusOperatorCRDs(ctx context.Context) error {
	version := getPrometheusOperatorVersion()
	crdURL := fmt.Sprintf("https://github.com/prometheus-operator/prometheus-operator/releases/download/%s/stripped-down-crds.yaml", version)

	fmt.Printf("Installing Prometheus Operator CRDs version %s...\n", version)

	// Apply CRDs using kubectl
	args := []string{
		"--kubeconfig", h.KubeConfigPath,
		"apply", "-f", crdURL,
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install Prometheus Operator CRDs from %s: %w", crdURL, err)
	}

	return nil
}

// Uninstall removes the Helm release
func (h *HelmInstaller) Uninstall(ctx context.Context, releaseName, namespace string) error {
	args := []string{
		"uninstall",
		releaseName,
		"-n", namespace,
	}

	cmd := exec.CommandContext(ctx, "helm", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", h.KubeConfigPath))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("helm uninstall failed: %w", err)
	}

	return nil
}
