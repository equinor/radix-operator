package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KindCluster represents a Kind cluster for testing
type KindCluster struct {
	Name           string
	KubeConfigPath string
}

// KindClusterConfig holds configuration for creating a Kind cluster
type KindClusterConfig struct {
	Name       string
	KubeConfig string
}

// NewKindCluster creates a new Kind cluster
func NewKindCluster(ctx context.Context, config KindClusterConfig) (*KindCluster, error) {
	cluster := &KindCluster{
		Name: config.Name,
	}

	// Set kubeconfig path
	if config.KubeConfig != "" {
		cluster.KubeConfigPath = config.KubeConfig
	} else {
		tmpDir, err := os.MkdirTemp("", "kind-kubeconfig-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp dir: %w", err)
		}
		cluster.KubeConfigPath = filepath.Join(tmpDir, "kubeconfig")
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

// create creates the Kind cluster
func (k *KindCluster) create(ctx context.Context) error {
	// Check if cluster already exists
	checkCmd := exec.CommandContext(ctx, "kind", "get", "clusters")
	output, _ := checkCmd.Output()
	
	// Delete existing cluster with same name
	if len(output) > 0 {
		deleteCmd := exec.CommandContext(ctx, "kind", "delete", "cluster", "--name", k.Name)
		_ = deleteCmd.Run()
	}

	// Create new cluster
	args := []string{
		"create", "cluster",
		"--name", k.Name,
		"--kubeconfig", k.KubeConfigPath,
		"--wait", "5m",
	}

	cmd := exec.CommandContext(ctx, "kind", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kind create cluster failed: %w", err)
	}

	return nil
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
