// ABOUTME: Unit tests for the ZenfraProvider schema and configuration.
// ABOUTME: Validates provider schema attributes and basic instantiation.
package provider

import (
	"testing"
)

func TestNewProvider(t *testing.T) {
	t.Parallel()

	factory := New("test")
	p := factory()
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestProviderSchema(t *testing.T) {
	t.Parallel()

	factory := New("test")
	p := factory()

	zp, ok := p.(*ZenfraProvider)
	if !ok {
		t.Fatal("expected *ZenfraProvider")
	}

	if zp.version != "test" {
		t.Errorf("expected version 'test', got %q", zp.version)
	}
}
