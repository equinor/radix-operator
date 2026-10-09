package internal

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path"
)

// Downloaded by `make bootstrap-e2e`
//
//go:embed manifests
var dependencyCRDs embed.FS

// InstallDependencyCRDs installs the third-party CRDs required by radix-operator
func InstallDependencyCRDs(ctx context.Context, kubeConfigPath string) error {
	entries, err := dependencyCRDs.ReadDir("manifests")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		file := entry.Name()
		fmt.Printf("Installing CRDs from %s...\n", file)

		manifest, err := dependencyCRDs.ReadFile(path.Join("manifests", file))
		if err != nil {
			return err
		}

		// Server-side apply avoids the last-applied-configuration annotation size limit on large CRDs
		cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeConfigPath, "apply", "--server-side", "-f", "-")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = bytes.NewReader(manifest)

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install CRDs from %s: %w", file, err)
		}
	}

	return nil
}
