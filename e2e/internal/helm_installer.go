package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime/debug"
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
	// Build helm install command
	args := []string{
		"install",
		config.ReleaseName,
		config.ChartPath,
		"--namespace", config.Namespace,
		"--create-namespace",
		"--wait",
		"--timeout", "5m",
	}

	// Add additional values
	for key, value := range config.Values {
		args = append(args, "--set", fmt.Sprintf("%s=%v", key, value))
	}

	// Install Helm chart
	cmd := exec.CommandContext(ctx, "helm", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", h.KubeConfigPath))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("helm install failed: %w", err)
	}

	return nil
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
