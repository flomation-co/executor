package helm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// A mirror serving the real archive bytes must install; serving anything else
// must be refused on the checksum, whatever TLS said.
func TestMirrorIsCheckedAgainstTheDeclaredChecksum(t *testing.T) {
	body := []byte("not a real helm tarball")
	sum := sha256.Sum256(body)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, ".tar.gz") {
			t.Errorf("unexpected mirror path %q", r.URL.Path)
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	t.Setenv("PATH", "/nonexistent")
	t.Setenv("FLOMATION_HELM_CACHE", t.TempDir())
	t.Setenv("FLOMATION_HELM_MIRROR", srv.URL)

	// Wrong checksum -> refuse.
	t.Setenv("FLOMATION_HELM_SHA256", strings.Repeat("ab", 32))
	if _, err := EnsureBinary(context.Background(), "9.9.9", ""); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("want a checksum mismatch, got %v", err)
	}

	// Right checksum -> gets past verification, then fails to find `helm` inside
	// (the body is not a real archive). That is the extraction step, i.e. the
	// checksum gate was passed.
	t.Setenv("FLOMATION_HELM_SHA256", hex.EncodeToString(sum[:]))
	_, err := EnsureBinary(context.Background(), "9.9.9", "")
	if err == nil || !strings.Contains(err.Error(), "not gzip") {
		t.Fatalf("want the archive to be read after a checksum match, got %v", err)
	}
}

func TestUnpinnedVersionNeedsAnExplicitDigest(t *testing.T) {
	t.Setenv("PATH", "/nonexistent")
	t.Setenv("FLOMATION_HELM_CACHE", t.TempDir())
	_, err := EnsureBinary(context.Background(), "9.9.9", "")
	if err == nil || !strings.Contains(err.Error(), "FLOMATION_HELM_SHA256") {
		t.Fatalf("want a refusal that names the escape hatch, got %v", err)
	}
}

func TestMalformedDigestIsRejected(t *testing.T) {
	t.Setenv("PATH", "/nonexistent")
	t.Setenv("FLOMATION_HELM_CACHE", t.TempDir())
	t.Setenv("FLOMATION_HELM_SHA256", "nope")
	if _, err := EnsureBinary(context.Background(), "9.9.9", ""); err == nil || !strings.Contains(err.Error(), "64-character hex") {
		t.Fatalf("want a digest-format refusal, got %v", err)
	}
}

func TestCABundleMustExist(t *testing.T) {
	t.Setenv("PATH", "/nonexistent")
	t.Setenv("FLOMATION_HELM_CACHE", t.TempDir())
	t.Setenv("FLOMATION_HELM_CA_BUNDLE", "/nonexistent/ca.pem")
	t.Setenv("FLOMATION_HELM_SHA256", strings.Repeat("ab", 32))
	if _, err := EnsureBinary(context.Background(), "9.9.9", ""); err == nil || !strings.Contains(err.Error(), "FLOMATION_HELM_CA_BUNDLE") {
		t.Fatalf("want a CA bundle error, got %v", err)
	}
	_ = os.Remove("unused")
}
