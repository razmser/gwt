package main

import (
	"testing"
)

func TestRepoName(t *testing.T) {
	tests := []struct {
		root     string
		expected string
	}{
		{"/Users/user/projects/gwt", "gwt"},
		{"/src/github.com/razmser/gwt", "gwt"},
		{"/", "/"},
	}

	for _, tt := range tests {
		got := repoName(tt.root)
		if got != tt.expected {
			t.Errorf("repoName(%q) = %q; want %q", tt.root, got, tt.expected)
		}
	}
}

func TestValidateWorktreeName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"valid", false},
		{"valid-name", false},
		{"valid_name", false},
		{"invalid/name", true},
		{"invalid name", true},
		{"", true},
		{".", true},
		{"..", true},
	}

	for _, tt := range tests {
		err := validateWorktreeName(tt.name)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateWorktreeName(%q) error = %v; wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}
