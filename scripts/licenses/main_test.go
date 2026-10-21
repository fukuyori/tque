package main

import (
	"runtime/debug"
	"testing"
)

func TestVerifyModules(t *testing.T) {
	modules := map[string]module{"example.org/lib": {Path: "example.org/lib", Version: "v1.2.3"}}
	for _, tt := range []struct {
		name      string
		deps      []*debug.Module
		wantError bool
	}{
		{"matching", []*debug.Module{{Path: "example.org/lib", Version: "v1.2.3"}}, false},
		{"missing", nil, true},
		{"wrong version", []*debug.Module{{Path: "example.org/lib", Version: "v1.2.4"}}, true},
		{"wrong module", []*debug.Module{{Path: "example.org/other", Version: "v1.2.3"}}, true},
		{"replacement", []*debug.Module{{Path: "example.org/lib", Version: "v1.2.3", Replace: &debug.Module{Path: "../local"}}}, true},
		{"extra", []*debug.Module{{Path: "example.org/lib", Version: "v1.2.3"}, {Path: "example.org/extra", Version: "v1.0.0"}}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := verifyModules(modules, tt.deps); (err != nil) != tt.wantError {
				t.Fatalf("verifyModules: %v", err)
			}
		})
	}
}
