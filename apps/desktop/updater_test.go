package main

import (
	"testing"
)

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		remote   string
		local    string
		expected bool
	}{
		{"1.6.1", "1.6.0", true},
		{"v1.7.0", "1.6.0", true},
		{"desktop-v2.0.0", "1.6.0", true},
		{"1.6.0-1", "1.6.0", false},
		{"1.6.0", "1.6.0", false},
		{"1.5.9", "1.6.0", false},
		{"1.6.0", "1.6.1", false},
		{"2.0.0", "1.9.9", true},
		{"1.10.0", "1.9.0", true},
	}

	for _, tt := range tests {
		got := isNewerVersion(tt.remote, tt.local)
		if got != tt.expected {
			t.Errorf("isNewerVersion(%q, %q) = %v; want %v", tt.remote, tt.local, got, tt.expected)
		}
	}
}
