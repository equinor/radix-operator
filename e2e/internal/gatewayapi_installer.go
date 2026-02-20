package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// InstallGatewayApiCRDs installs the Gateway API CRDs from GitHub
func InstallGatewayApiCRDs(ctx context.Context, KubeConfigPath string) error {
	crdURL := "https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.4.0/experimental-install.yaml"

	fmt.Print("Installing Gateway API CRDs...\n")

	// Apply CRDs using kubectl
	args := []string{
		"--kubeconfig", KubeConfigPath,
		"apply", "--server-side", "-f", crdURL,
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install Gateway API CRDs from %s: %w", crdURL, err)
	}

	return nil
}
