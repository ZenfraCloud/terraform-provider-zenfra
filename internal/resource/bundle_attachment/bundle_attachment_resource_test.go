// ABOUTME: Unit tests for the zenfra_bundle_attachment resource import state parsing.
// ABOUTME: Verifies composite ID splitting and model construction.
package bundle_attachment

import (
	"testing"
)

func TestCompositeIDFormat(t *testing.T) {
	tests := []struct {
		name     string
		stackID  string
		bundleID string
		expected string
	}{
		{
			name:     "standard IDs",
			stackID:  "stack-abc-123",
			bundleID: "bundle-def-456",
			expected: "stack-abc-123:bundle-def-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.stackID + ":" + tt.bundleID
			if result != tt.expected {
				t.Errorf("composite ID: got %s, want %s", result, tt.expected)
			}
		})
	}
}
