//go:build tools

// This package imports things required by build scripts, to force `go mod` to see them as dependencies
// https://go.dev/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
package tools

import (
	_ "k8s.io/code-generator" // Used for generating typed Kubernetes client for Radix custom resources.
)
