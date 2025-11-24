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
