package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// GenerateImageTag generates a unique tag for the current build
func GenerateImageTag() string {
	// Get current timestamp for uniqueness
	return fmt.Sprintf("e2e-%s", time.Now().Format("20060102-150405"))
}

// BuildImage builds a single Docker image
func BuildImage(ctx context.Context, dockerfile, imageName, imageTag string) error {
	// Get the project root directory (parent of e2e directory)
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	projectRoot := filepath.Dir(cwd)

	fullImageName := fmt.Sprintf("%s:%s", imageName, imageTag)
	fmt.Printf("Building image %s...\n", fullImageName)

	// Build the Docker image from project root
	buildCmd := exec.CommandContext(ctx, "docker", "build",
		"-t", fullImageName,
		"-f", dockerfile,
		".",
	)
	buildCmd.Dir = projectRoot
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("failed to build image %s: %w", fullImageName, err)
	}

	fmt.Printf("Successfully built %s\n", fullImageName)
	return nil
}

// RemoveImage removes a Docker image
func RemoveImage(ctx context.Context, imageName, imageTag string) error {
	fullImageName := fmt.Sprintf("%s:%s", imageName, imageTag)
	fmt.Printf("Removing image %s\n", fullImageName)

	// Remove docker image
	buildCmd := exec.CommandContext(ctx, "docker", "image", "rm", fullImageName)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("failed to remove image %s: %w", fullImageName, err)
	}

	fmt.Printf("Successfully removed %s\n", fullImageName)
	return nil
}

// PruneBuildCache prunes build cache
func PruneBuildCache(ctx context.Context) error {
	fmt.Println("Prune build cache")

	// Build the Docker image from project root
	buildCmd := exec.CommandContext(ctx, "docker", "builder", "prune", "--force")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("failed to prune build cache: %w", err)
	}

	fmt.Println("Successfully pruned build cache")
	return nil
}
