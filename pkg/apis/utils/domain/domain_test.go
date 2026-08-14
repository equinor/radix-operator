package domain

import "testing"

func TestGetComponentHostname(t *testing.T) {
	if actual, expected := GetComponentHostname("component", "namespace"), "component-namespace"; actual != expected {
		t.Errorf("GetComponentHostname() = %q, want %q", actual, expected)
	}
}
