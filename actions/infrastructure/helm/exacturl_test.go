package helm

// End-to-end coverage for fetching helm from an internal artefact host.
//
// Each test serves a real artefact over a real HTTP server, lets EnsureBinary
// download → verify → unwrap → install it, and then EXECUTES what landed. The
// "helm" inside is a shell script that prints a marker, so a test that merely
// wrote a file to the right place, or extracted the wrong entry, still fails.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const fakeHelm = "#!/bin/sh\necho FAKE-HELM-OK\n"

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// tarGz builds the shape upstream ships: <goos>-<arch>/helm inside a gzip tar,
// alongside the other files a real release carries.
func tarGz(t *testing.T, entryPath string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	write := func(name, body string) {
		if err := tw.WriteHeader(&tar.Header{
			Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	// A decoy that must NOT be installed: same directory, different name.
	write("linux-amd64/LICENSE", "not the binary")
	write(entryPath, fakeHelm)

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func serve(t *testing.T, path string, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func mustRun(t *testing.T, bin string) string {
	t.Helper()
	out, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("running the installed binary %s: %v (%s)", bin, err, out)
	}
	return strings.TrimSpace(string(out))
}

// An internal host serving the archive under its own name — the exact case in
// "https://example.com/helm/helm.tgz". The digest is of the repackaged archive,
// so it must be declared.
func TestExactURLInstallsFromArchive(t *testing.T) {
	archive := tarGz(t, "linux-amd64/helm")
	srv := serve(t, "/helm/helm.tgz", archive)

	t.Setenv("PATH", "/nonexistent")
	t.Setenv("FLOMATION_HELM_CACHE", t.TempDir())
	t.Setenv("FLOMATION_HELM_URL", srv.URL+"/helm/helm.tgz")
	t.Setenv("FLOMATION_HELM_SHA256", sum(archive))

	bin, err := EnsureBinary(context.Background(), "", "")
	if err != nil {
		t.Fatalf("EnsureBinary: %v", err)
	}
	if got := mustRun(t, bin); got != "FAKE-HELM-OK" {
		t.Fatalf("installed the wrong entry: ran %q, got %q", bin, got)
	}
}

// The archive's internal layout is upstream's business, not ours: the binary is
// found by basename wherever it sits.
func TestExactURLFindsHelmAtAnyDepth(t *testing.T) {
	archive := tarGz(t, "some/other/place/helm")
	srv := serve(t, "/artifacts/helm.tgz", archive)

	t.Setenv("PATH", "/nonexistent")
	t.Setenv("FLOMATION_HELM_CACHE", t.TempDir())
	t.Setenv("FLOMATION_HELM_URL", srv.URL+"/artifacts/helm.tgz")
	t.Setenv("FLOMATION_HELM_SHA256", sum(archive))

	bin, err := EnsureBinary(context.Background(), "", "")
	if err != nil {
		t.Fatalf("EnsureBinary: %v", err)
	}
	if got := mustRun(t, bin); got != "FAKE-HELM-OK" {
		t.Fatalf("got %q", got)
	}
}

// Plenty of internal hosts serve the naked executable rather than an archive.
// The artefact type is decided by gzip's magic bytes, not the URL's extension.
func TestExactURLInstallsRawBinary(t *testing.T) {
	raw := []byte(fakeHelm)
	// Deliberately named .tgz while being no such thing, to prove the sniff wins.
	srv := serve(t, "/helm/helm.tgz", raw)

	t.Setenv("PATH", "/nonexistent")
	t.Setenv("FLOMATION_HELM_CACHE", t.TempDir())
	t.Setenv("FLOMATION_HELM_URL", srv.URL+"/helm/helm.tgz")
	t.Setenv("FLOMATION_HELM_SHA256", sum(raw))

	bin, err := EnsureBinary(context.Background(), "", "")
	if err != nil {
		t.Fatalf("EnsureBinary: %v", err)
	}
	if got := mustRun(t, bin); got != "FAKE-HELM-OK" {
		t.Fatalf("got %q", got)
	}
}

// The checksum gate holds on the exact-URL path too: a host serving different
// bytes than the operator declared installs nothing.
func TestExactURLStillEnforcesTheChecksum(t *testing.T) {
	srv := serve(t, "/helm.tgz", []byte("something else entirely"))

	t.Setenv("PATH", "/nonexistent")
	t.Setenv("FLOMATION_HELM_CACHE", t.TempDir())
	t.Setenv("FLOMATION_HELM_URL", srv.URL+"/helm.tgz")
	t.Setenv("FLOMATION_HELM_SHA256", sum([]byte(fakeHelm)))

	_, err := EnsureBinary(context.Background(), "", "")
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("want a checksum mismatch, got %v", err)
	}
}

// Without a declared digest, a repackaged artefact is compared against the
// PINNED upstream digest, fails, and the error says what to do about it.
func TestRepackagedArtefactWithoutDigestIsRefusedWithAHint(t *testing.T) {
	archive := tarGz(t, "linux-amd64/helm")
	srv := serve(t, "/helm.tgz", archive)

	t.Setenv("PATH", "/nonexistent")
	t.Setenv("FLOMATION_HELM_CACHE", t.TempDir())
	t.Setenv("FLOMATION_HELM_URL", srv.URL+"/helm.tgz")

	_, err := EnsureBinary(context.Background(), DefaultVersion, "")
	if err == nil {
		t.Fatal("want a refusal")
	}
	for _, want := range []string{"checksum mismatch", "FLOMATION_HELM_SHA256", srv.URL} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not mention %q", err, want)
		}
	}
}

// URL beats MIRROR, and both beat the official host.
func TestExactURLWinsOverMirror(t *testing.T) {
	raw := []byte(fakeHelm)
	srv := serve(t, "/exact", raw)

	t.Setenv("PATH", "/nonexistent")
	t.Setenv("FLOMATION_HELM_CACHE", t.TempDir())
	t.Setenv("FLOMATION_HELM_MIRROR", "https://mirror.invalid/helm")
	t.Setenv("FLOMATION_HELM_URL", srv.URL+"/exact")
	t.Setenv("FLOMATION_HELM_SHA256", sum(raw))

	bin, err := EnsureBinary(context.Background(), "", "")
	if err != nil {
		t.Fatalf("EnsureBinary should have used the exact URL, not the (invalid) mirror: %v", err)
	}
	if got := mustRun(t, bin); got != "FAKE-HELM-OK" {
		t.Fatalf("got %q", got)
	}
}

func TestExactURLRejectsNonHTTPSchemes(t *testing.T) {
	for _, bad := range []string{"file:///etc/passwd", "ftp://host/helm.tgz", "not a url"} {
		t.Setenv("PATH", "/nonexistent")
		t.Setenv("FLOMATION_HELM_CACHE", t.TempDir())
		t.Setenv("FLOMATION_HELM_URL", bad)
		t.Setenv("FLOMATION_HELM_SHA256", strings.Repeat("ab", 32))

		_, err := EnsureBinary(context.Background(), "", "")
		if err == nil || !strings.Contains(err.Error(), "FLOMATION_HELM_URL") {
			t.Fatalf("%s: want a URL refusal, got %v", bad, err)
		}
	}
}

// A host on PATH still wins over any download configuration — the air-gapped
// answer stays the simplest one.
func TestPathStillWinsOverExactURL(t *testing.T) {
	dir := t.TempDir()
	if err := writeExecutable(dir+"/helm", fakeHelm); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("FLOMATION_HELM_URL", "https://never.invalid/helm.tgz")

	bin, err := EnsureBinary(context.Background(), "", "")
	if err != nil {
		t.Fatalf("EnsureBinary: %v", err)
	}
	if bin != dir+"/helm" {
		t.Fatalf("resolved %q, want the binary on PATH", bin)
	}
}

func writeExecutable(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o755) // #nosec G306 -- a test fixture that must be executable
}
