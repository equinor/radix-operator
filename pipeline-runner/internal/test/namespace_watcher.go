package test

// FakeNamespaceWatcher Unit tests doesn't handle muliti-threading well
type FakeNamespaceWatcher struct {
}

// WaitFor Waits for namespace to appear
func (watcher FakeNamespaceWatcher) WaitFor(namespace string) error {
	return nil
}
