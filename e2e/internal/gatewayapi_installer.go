package internal

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
)

//go:embed gateway-api-standard-install.yaml
var GatewayAPIManifest []byte

// InstallGatewayApiCRDs installs the Gateway API CRDs from GitHub
func InstallGatewayApiCRDs(ctx context.Context, KubeConfigPath string) error {

	fmt.Print("Installing Gateway API CRDs...\n")

	// Apply CRDs using kubectl
	args := []string{
		"--kubeconfig", KubeConfigPath,
		"apply", "--server-side",
		"-f", "-", // read from stdin
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = bytes.NewReader(GatewayAPIManifest)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install Gateway API CRDs: %w", err)
	}

	return nil
}
