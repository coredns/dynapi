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
	t.Parallel()

	binary := os.Getenv("COREDNS_DYNAPI_BINARY")
	if binary == "" {
		t.Fatal("COREDNS_DYNAPI_BINARY is required. Run make integration.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)

	defer cancel()

	//nolint:gosec // The test runner supplies the CoreDNS executable.
	output, err := exec.CommandContext(ctx, binary, "-plugins").CombinedOutput()
	if err != nil {
		t.Fatalf("listing plugins: %v\n%s", err, output)
	}

	for _, plugin := range []string{"dynapi", "dynupdate", "tsig"} {
		if !slices.Contains(strings.Fields(string(output)), plugin) {
			t.Fatalf("%s is missing from the built executable:\n%s", plugin, output)
		}
	}

	corefile := filepath.Join(t.TempDir(), "Corefile")
	if writeErr := os.WriteFile(
		corefile,
		[]byte("example.org:1053 {\n    dynapi\n}\n"),
		0o600,
	); writeErr != nil {
		t.Fatal(writeErr)
	}

	//nolint:gosec // The test runner supplies the CoreDNS executable.
	output, err = exec.CommandContext(ctx, binary, "-conf", corefile).CombinedOutput()
	if err == nil {
		t.Fatal("an unconfigured dynapi directive unexpectedly started")
	}

	if ctx.Err() != nil {
		t.Fatalf("CoreDNS did not reject configuration before the timeout: %v", ctx.Err())
	}

	if !strings.Contains(string(output), "configure token or token_env") {
		t.Fatalf("unexpected startup failure: %v\n%s", err, output)
	}
}
