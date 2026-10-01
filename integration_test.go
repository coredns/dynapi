//go:build integration

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestInstallation(t *testing.T) {
	binary := os.Getenv("COREDNS_DYNAPI_BINARY")
	if binary == "" {
		t.Fatal("COREDNS_DYNAPI_BINARY is required. Run make integration.")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, binary, "-plugins").CombinedOutput()
	if err != nil {
		t.Fatalf("listing plugins: %v\n%s", err, output)
	}
	if !slices.Contains(strings.Fields(string(output)), "dynapi") {
		t.Fatalf("dynapi is missing from the built executable:\n%s", output)
	}
	corefile := filepath.Join(t.TempDir(), "Corefile")
	if err := os.WriteFile(corefile, []byte("example.org:1053 {\n    dynapi\n}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	output, err = exec.CommandContext(ctx, binary, "-conf", corefile).CombinedOutput()
	if err == nil {
		t.Fatal("the unfinished dynapi directive unexpectedly started")
	}
	if ctx.Err() != nil {
		t.Fatalf("CoreDNS did not reject configuration before the timeout: %v", ctx.Err())
	}
	if !strings.Contains(string(output), "plugin/dynapi: HTTP record management is not implemented yet") {
		t.Fatalf("unexpected startup failure: %v\n%s", err, output)
	}
}
