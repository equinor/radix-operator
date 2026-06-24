package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime/debug"
)

// InstallPrometheusOperatorCRDs installs the Prometheus Operator CRDs from GitHub
func InstallPrometheusOperatorCRDs(ctx context.Context, KubeConfigPath string) error {
	version := getPrometheusOperatorVersion()
	crdURL := fmt.Sprintf("https://github.com/prometheus-operator/prometheus-operator/releases/download/%s/stripped-down-crds.yaml", version)

	fmt.Printf("Installing Prometheus Operator CRDs version %s...\n", version)

	// Apply CRDs using kubectl
	args := []string{
		"--kubeconfig", KubeConfigPath,
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
