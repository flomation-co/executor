package helm

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestRefusesUnpinnedVersion(t *testing.T) {
	t.Setenv("PATH", "/nonexistent")
	t.Setenv("FLOMATION_HELM_CACHE", t.TempDir())
	_, err := EnsureBinary(context.Background(), "3.0.0-not-pinned", "")
	if err == nil || !strings.Contains(err.Error(), "no checksum is pinned") {
		t.Fatalf("want a pinned-checksum refusal, got %v", err)
	}
}

func TestDownloadDisabledByPolicy(t *testing.T) {
	t.Setenv("PATH", "/nonexistent")
	t.Setenv("FLOMATION_HELM_CACHE", t.TempDir())
	t.Setenv("FLOMATION_HELM_DISABLE_DOWNLOAD", "1")
	_, err := EnsureBinary(context.Background(), "", "")
	if err == nil || !strings.Contains(err.Error(), "runtime download is disabled") {
		t.Fatalf("want a policy refusal, got %v", err)
	}
}

func TestBinaryPathOverrideMustBeExecutable(t *testing.T) {
	f, _ := os.CreateTemp(t.TempDir(), "notexec")
	_, err := EnsureBinary(context.Background(), "", f.Name())
	if err == nil || !strings.Contains(err.Error(), "not an executable file") {
		t.Fatalf("want a non-executable refusal, got %v", err)
	}
}
