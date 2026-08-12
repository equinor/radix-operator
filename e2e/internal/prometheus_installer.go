package internal

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
)

//go:embed prometheus-operator-stripped-down-crds.yaml
var PrometheusOperatorCRDs []byte

// InstallPrometheusOperatorCRDs installs the Prometheus Operator CRDs from GitHub
func InstallPrometheusOperatorCRDs(ctx context.Context, KubeConfigPath string) error {

	fmt.Printf("Installing Prometheus Operator CRDs...\n")

	// Apply CRDs using kubectl
	args := []string{
		"--kubeconfig", KubeConfigPath,
		"apply", "-f", "-",
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = bytes.NewReader(PrometheusOperatorCRDs)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install Prometheus Operator CRDs: %w", err)
	}

	return nil
}
