// Binary resolution for the Helm actions: find a usable `helm`, or fetch the
// pinned release and verify it before trusting it.
//
// This mirrors the OpenTofu actions' tofu package (actions/opentofu/tofu), with
// one deliberate difference in the trust chain.
//
// OpenTofu publishes a detached GPG signature over its SHA256SUMS file, so that
// package can fetch the checksums at run time and still know they are authentic:
// the embedded signing key vouches for them. Helm publishes no such signature —
// only a <artifact>.tar.gz.sha256sum served from the same host as the tarball.
// Fetching that checksum would prove only that the bytes we received are the
// bytes get.helm.sh served, which is what TLS already tells us; a compromised
// mirror would happily serve a matching pair.
//
// So the checksums are pinned HERE, in source, reviewed like any other code. A
// version we have no pinned checksum for is never downloaded: the operator is
// told to install helm on the runner or point binary_path at it. That is a
// stricter policy than OpenTofu's, and it is the right one given what Helm ships.
package helm

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// DefaultVersion is the Helm release downloaded when none is on PATH.
//
// Bumping it: change this constant AND add the matching entry to
// pinnedChecksums. Fetch them with, for each platform:
//
//	curl -fsSL https://get.helm.sh/helm-v<ver>-<goos>-<arch>.tar.gz.sha256sum
//
// Helm 3 is pinned rather than 4 because its client is compatible with the
// widest span of API-server versions. A host that provides its own helm — of
// either major version — is preferred over this download, and the flags these
// actions use are identical across both.
const DefaultVersion = "3.21.3"

const binaryName = "helm"

// pinnedChecksums maps version → "<goos>-<arch>" → SHA-256 of the release
// tarball. Verified against get.helm.sh at the time the version was pinned.
var pinnedChecksums = map[string]map[string]string{
	"3.21.3": {
		"linux-amd64":  "15e041a93a590dce8100f39385cd98c84a765c9e36aeeb9e2dc6ff9e4769e2e0",
		"linux-arm64":  "67f58155079ff9ffab98ba5c88daff0ed9b542f3a4732f5dd426dde7dd0f5244",
		"darwin-amd64": "76d0db4730b05d3d625eee11e80f0721b32b4d8422f4e5d093de6337bf3ac9f8",
		"darwin-arm64": "19879a848cad832b7a1ac24b767a481d20fb3b95ab53a220849649422ada144e",
	},
}

// maxBinarySize bounds the download. The helm tarball is ~17 MB; 128 MB leaves
// generous headroom while refusing to stream an unbounded body to disk.
const maxBinarySize = 128 << 20

// downloadMu serialises downloads within a process so concurrent flows resolving
// the same version don't race on the cache. Cross-process safety rests on the
// atomic rename at the end of extractBinary.
var downloadMu sync.Mutex

var httpClient = &http.Client{Timeout: 5 * time.Minute}

// EnsureBinary returns a path to a usable `helm`. Resolution order, most to
// least preferred:
//
//  1. override — an explicit operator-chosen binary path.
//  2. A `helm` already on PATH: provisioned by the host image or a package.
//     Used as-is, whatever its version.
//  3. A previously-downloaded binary cached for the requested version.
//  4. A fresh download of a version with a pinned checksum, verified against it.
//
// Set FLOMATION_HELM_DISABLE_DOWNLOAD=1 to forbid step 4 by policy — the right
// setting for an air-gapped runner that must use a host-provisioned binary.
func EnsureBinary(ctx context.Context, version, override string) (string, error) {
	if override != "" {
		if !isExecutableFile(override) {
			return "", fmt.Errorf("helm binary_path %q is not an executable file", override)
		}
		return override, nil
	}

	if p, err := exec.LookPath(binaryName); err == nil {
		return p, nil
	}

	if version == "" {
		version = DefaultVersion
	}
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")

	dest := cachePath(version)
	if isExecutableFile(dest) {
		return dest, nil
	}

	if downloadDisabled() {
		return "", fmt.Errorf("no %s binary found on PATH and runtime download is disabled "+
			"(FLOMATION_HELM_DISABLE_DOWNLOAD): install Helm on the runner, or set binary_path", binaryName)
	}

	platform := runtime.GOOS + "-" + runtime.GOARCH
	sums, ok := pinnedChecksums[version]
	if !ok {
		return "", fmt.Errorf("refusing to download helm %s: no checksum is pinned for it. "+
			"Install Helm on the runner, set binary_path, or use the pinned version %s", version, DefaultVersion)
	}
	want, ok := sums[platform]
	if !ok {
		return "", fmt.Errorf("refusing to download helm %s: no checksum is pinned for %s. "+
			"Install Helm on the runner, or set binary_path", version, platform)
	}

	downloadMu.Lock()
	defer downloadMu.Unlock()

	// Re-check under the lock: another goroutine may have just finished.
	if isExecutableFile(dest) {
		return dest, nil
	}
	if err := download(ctx, version, platform, want, dest); err != nil {
		return "", fmt.Errorf("could not obtain helm %s: %w", version, err)
	}
	return dest, nil
}

func downloadDisabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("FLOMATION_HELM_DISABLE_DOWNLOAD"))) {
	case "1", "true", "yes":
		return true
	}
	return false
}

func cacheRoot() string {
	if d := os.Getenv("FLOMATION_HELM_CACHE"); d != "" {
		return d
	}
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "flomation", "helm")
}

func cachePath(version string) string {
	return filepath.Join(cacheRoot(), version, binaryName)
}

func isExecutableFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular() && fi.Mode()&0o111 != 0
}

// download fetches the release tarball, checks it against the pinned checksum,
// and installs the binary at dest.
func download(ctx context.Context, version, platform, wantSum, dest string) error {
	url := fmt.Sprintf("https://get.helm.sh/helm-v%s-%s.tar.gz", version, platform)

	tmp, err := os.CreateTemp("", "helm-download-*.tar.gz")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	defer func() { _ = tmp.Close() }()

	gotSum, err := fetchToFile(ctx, url, tmp)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	if !strings.EqualFold(gotSum, wantSum) {
		return fmt.Errorf("checksum mismatch for helm %s (%s): got %s, want %s — "+
			"refusing to install", version, platform, gotSum, wantSum)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return extractBinary(tmp.Name(), dest)
}

// fetchToFile streams url into f and returns the hex SHA-256 of what was written.
func fetchToFile(ctx context.Context, url string, f *os.File) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: unexpected status %s", url, resp.Status)
	}

	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, maxBinarySize+1))
	if err != nil {
		return "", err
	}
	if n > maxBinarySize {
		return "", fmt.Errorf("helm archive exceeds %d bytes", maxBinarySize)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// extractBinary pulls the `helm` entry out of the release tarball (which nests
// it under <goos>-<arch>/helm) and installs it at dest via temp-file-and-rename,
// so a concurrent or aborted extraction can never leave a half-written binary in
// the cache for another process to execute.
func extractBinary(tarPath, dest string) error {
	f, err := os.Open(tarPath) // #nosec G304 -- path is our own CreateTemp result
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("release archive is not gzip: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != binaryName {
			continue
		}

		tmp, err := os.CreateTemp(filepath.Dir(dest), ".helm-*")
		if err != nil {
			return err
		}
		tmpName := tmp.Name()

		// The archive is checksum-verified before we get here, so its declared
		// size is trusted; the limit is belt-and-braces against a malformed header.
		if _, err := io.Copy(tmp, io.LimitReader(tr, maxBinarySize)); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
			return err
		}
		if err := tmp.Chmod(0o755); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
			return err
		}
		if err := tmp.Close(); err != nil {
			_ = os.Remove(tmpName)
			return err
		}
		return os.Rename(tmpName, dest)
	}
	return fmt.Errorf("%s binary not found inside the release archive", binaryName)
}
