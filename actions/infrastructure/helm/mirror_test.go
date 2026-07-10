package helm

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
)

// The checksum gate runs before the artefact is unwrapped, so a mirror serving
// bytes the operator did not declare installs nothing — whatever TLS said, and
// whatever the URL's filename implied.
//
// It also pins that the mirror is a DIRECTORY: the canonical upstream filename
// is appended to it. (FLOMATION_HELM_URL is the exact-URL form; see exacturl_test.)
func TestMirrorIsCheckedAgainstTheDeclaredChecksum(t *testing.T) {
	body := []byte("#!/bin/sh\necho mirrored\n")
	sum := sha256.Sum256(body)

	var requested string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = r.URL.Path
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	t.Setenv("PATH", "/nonexistent")
	t.Setenv("FLOMATION_HELM_CACHE", t.TempDir())
	t.Setenv("FLOMATION_HELM_MIRROR", srv.URL)

	// Wrong checksum -> refuse, and install nothing.
	t.Setenv("FLOMATION_HELM_SHA256", strings.Repeat("ab", 32))
	if _, err := EnsureBinary(context.Background(), "9.9.9", ""); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("want a checksum mismatch, got %v", err)
	}
	if !strings.HasSuffix(requested, "/helm-v9.9.9-"+runtime.GOOS+"-"+runtime.GOARCH+".tar.gz") {
		t.Fatalf("mirror should have the canonical filename appended, requested %q", requested)
	}

	// Right checksum -> installed, and what landed is exactly what was served.
	t.Setenv("FLOMATION_HELM_SHA256", hex.EncodeToString(sum[:]))
	bin, err := EnsureBinary(context.Background(), "9.9.9", "")
	if err != nil {
		t.Fatalf("EnsureBinary after a checksum match: %v", err)
	}
	got, err := os.ReadFile(bin) // #nosec G304 -- EnsureBinary's own result
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("installed %q, want the served bytes", got)
	}
	if fi, err := os.Stat(bin); err != nil || fi.Mode()&0o111 == 0 {
		t.Fatalf("installed binary is not executable (mode %v, err %v)", fi.Mode(), err)
	}
}

// A gzip stream that is not a tar, or a tar with no `helm` in it, is a broken
// artefact rather than a naked binary — say so rather than installing a tarball
// as if it were an executable.
func TestGzipWithoutHelmInsideIsRejected(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write([]byte("gzipped, but not a tar"))
	_ = gz.Close()
	body := buf.Bytes()
	sum := sha256.Sum256(body)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	t.Setenv("PATH", "/nonexistent")
	t.Setenv("FLOMATION_HELM_CACHE", t.TempDir())
	t.Setenv("FLOMATION_HELM_URL", srv.URL+"/helm.tgz")
	t.Setenv("FLOMATION_HELM_SHA256", hex.EncodeToString(sum[:]))

	if _, err := EnsureBinary(context.Background(), "", ""); err == nil {
		t.Fatal("want a rejection for a gzip artefact containing no helm binary")
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
