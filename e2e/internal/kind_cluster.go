package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KindCluster represents a Kind cluster for testing
type KindCluster struct {
	Name           string
	KubeConfigPath string
	// Existed is true if the cluster already existed and was reused instead of created.
	Existed bool
}

// NewKindCluster creates a new Kind cluster
func NewKindCluster(ctx context.Context) (*KindCluster, error) {
	tmpDir, err := os.MkdirTemp("", "kind-kubeconfig-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	cluster := &KindCluster{
		Name:           "radix-operator-e2e",
		KubeConfigPath: filepath.Join(tmpDir, "kubeconfig"),
	}

	// Create Kind cluster
	if err := cluster.create(ctx); err != nil {
		return nil, fmt.Errorf("failed to create kind cluster: %w", err)
	}

	// Wait for cluster to be ready
	if err := cluster.waitForReady(ctx); err != nil {
		_ = cluster.Delete(ctx)
		return nil, fmt.Errorf("cluster not ready: %w", err)
	}

	return cluster, nil
}

// create creates the Kind cluster, or reuses it if a cluster with the same name already exists
func (k *KindCluster) create(ctx context.Context) error {
	exists, err := k.exists(ctx)
	if err != nil {
		return err
	}

	// Reuse the existing cluster instead of recreating it
	if exists {
		log.Ctx(ctx).Info().Msgf("Kind cluster %q already exists, reusing it\n", k.Name)
		k.Existed = true

		exportCmd := exec.CommandContext(ctx, "kind", "export", "kubeconfig", "--name", k.Name, "--kubeconfig", k.KubeConfigPath)
		exportCmd.Stdout = os.Stdout
		exportCmd.Stderr = os.Stderr
		if err := exportCmd.Run(); err != nil {
			return fmt.Errorf("failed to export kubeconfig for existing cluster: %w", err)
		}
		return nil
	}

	// Create new cluster
	cmd := exec.CommandContext(ctx, "kind", "create", "cluster", "--name", k.Name, "--kubeconfig", k.KubeConfigPath, "--wait", "5m")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kind create cluster failed: %w", err)
	}

	return nil
}

// exists returns true if a Kind cluster with this name already exists
func (k *KindCluster) exists(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "kind", "get", "clusters")
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to list kind clusters: %w", err)
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == k.Name {
			return true, nil
		}
	}
	return false, nil
}

// waitForReady waits for the cluster to be ready
func (k *KindCluster) waitForReady(ctx context.Context) error {
	timeout := 5 * time.Minute
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", k.KubeConfigPath, "get", "nodes")
		if err := cmd.Run(); err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
			continue
		}
	}

	return fmt.Errorf("cluster not ready after %v", timeout)
}

// Delete removes the Kind cluster
func (k *KindCluster) Delete(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "kind", "delete", "cluster", "--name", k.Name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete kind cluster: %w", err)
	}

	// Clean up kubeconfig
	if k.KubeConfigPath != "" {
		tmpDir := filepath.Dir(k.KubeConfigPath)
		_ = os.RemoveAll(tmpDir)
	}

	return nil
}

// GetKubeConfig returns the rest.Config for the cluster
func (k *KindCluster) GetKubeConfig() (*rest.Config, error) {
	config, err := clientcmd.BuildConfigFromFlags("", k.KubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}
	return config, nil
}

// LoadImage loads a single Docker image into the Kind cluster
func (k *KindCluster) LoadImage(ctx context.Context, image, tag string) error {
	imageName := fmt.Sprintf("%s:%s", image, tag)
	log.Info().Msgf("Loading image %s into Kind cluster...\n", imageName)

	// Load the image into Kind
	loadCmd := exec.CommandContext(ctx, "kind", "load", "docker-image", "--name", k.Name, imageName)
	loadCmd.Stdout = os.Stdout
	loadCmd.Stderr = os.Stderr

	if err := loadCmd.Run(); err != nil {
		return fmt.Errorf("failed to load %s into kind: %w", imageName, err)
	}

	log.Info().Msgf("Successfully loaded %s\n", imageName)
	return nil
}
