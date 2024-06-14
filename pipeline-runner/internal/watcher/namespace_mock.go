package watcher

import "context"

// FakeNamespaceWatcher Unit tests doesn't handle multi-threading well
type FakeNamespaceWatcher struct {
}

// FakeRadixDeploymentWatcher Unit tests doesn't handle multi-threading well
type FakeRadixDeploymentWatcher struct {
}

// WaitFor Waits for namespace to appear
func (watcher FakeNamespaceWatcher) WaitFor(_ context.Context, _ string) error {
	return nil
}

// WaitFor Waits for radix deployment gets active
func (watcher FakeRadixDeploymentWatcher) WaitForActive(_, _ string) error {
	return nil
}
