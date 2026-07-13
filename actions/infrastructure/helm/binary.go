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
// bytes the host served, which is what TLS already tells us; a compromised
// mirror would happily serve a matching pair.
//
// So the checksums are pinned HERE, in source, reviewed like any other code, and
// a binary is never installed unless its SHA-256 matches. That invariant holds on
// every path below, which is what makes the mirror and TLS overrides safe: an
// operator may redirect the download to an internal Nexus, present a private CA,
// or even switch verification off — the bytes still have to match a checksum the
// operator or this file declared in advance.
//
// # Air-gapped and corporate environments
//
// Resolution never touches the network when a binary is already present, so the
// simplest answer for an air-gapped runner remains: install helm on the host (or
// bake it into the image) and it is used as-is. Beyond that, these environment
// variables on the runner shape the download:
//
//	FLOMATION_HELM_DISABLE_DOWNLOAD=1   forbid runtime downloads entirely
//	FLOMATION_HELM_URL=<url>            the exact URL of the artefact, whatever
//	                                    it is named — https://nexus.corp/helm.tgz
//	                                    or a naked binary. Wins over MIRROR.
//	FLOMATION_HELM_MIRROR=<base-url>    a directory the canonical upstream
//	                                    filename hangs off:
//	                                    <mirror>/helm-v<ver>-<goos>-<arch>.tar.gz
//	FLOMATION_HELM_SHA256=<hex>         the expected digest of whatever the two
//	                                    above resolve to. Required for a version
//	                                    this file does not pin, or for an artefact
//	                                    repackaged rather than mirrored verbatim.
//	FLOMATION_HELM_CA_BUNDLE=<path>     PEM bundle used to verify that host,
//	                                    for a Nexus behind an internal CA
//	FLOMATION_HELM_INSECURE=1           skip TLS verification of that host
//
// URL and MIRROR differ only in who names the file. A mirror that preserves
// upstream's filename needs no SHA256 override: the bytes, and therefore the
// digest, are unchanged. An artefact repackaged under a new name has a different
// digest and must declare it.
//
// They are environment variables rather than node inputs because which binary a
// runner may execute, and which CA it trusts, is a property of the host — not of
// a flow an operator drew.
package helm

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// downloadClient builds the client used to fetch the release archive. It honours
// FLOMATION_HELM_CA_BUNDLE (an internal CA, for a mirror behind a corporate PKI)
// and FLOMATION_HELM_INSECURE (skip verification altogether).
//
// Switching verification off does not make the download unverified: the archive
// is still checked against a checksum pinned in this file or supplied through
// FLOMATION_HELM_SHA256, and a mismatch refuses to install. TLS protects the
// transport; the checksum protects the artefact.
func downloadClient() (*http.Client, error) {
	// #nosec G402 -- InsecureSkipVerify is an explicit operator opt-in for a
	// mirror behind an internal CA. Integrity is enforced by the checksum, which
	// no path below can skip.
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}

	if insecureDownload() {
		tlsCfg.InsecureSkipVerify = true
	} else if bundle := strings.TrimSpace(os.Getenv("FLOMATION_HELM_CA_BUNDLE")); bundle != "" {
		pem, err := os.ReadFile(bundle) // #nosec G304 -- an operator-configured trust store path
		if err != nil {
			return nil, fmt.Errorf("could not read FLOMATION_HELM_CA_BUNDLE %q: %w", bundle, err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("FLOMATION_HELM_CA_BUNDLE %q contains no usable PEM certificate", bundle)
		}
		tlsCfg.RootCAs = pool
	}

	return &http.Client{
		Timeout:   5 * time.Minute,
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
	}, nil
}

func insecureDownload() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("FLOMATION_HELM_INSECURE"))) {
	case "1", "true", "yes":
		return true
	}
	return false
}

// releaseURL is where the artefact is fetched from. Resolution order:
//
//  1. FLOMATION_HELM_URL — the exact URL of the artefact, whatever it is called.
//     Use this when an internal host serves the file under its own name
//     (https://nexus.corp/raw/helm.tgz) rather than mirroring upstream's.
//  2. FLOMATION_HELM_MIRROR — a directory the canonical upstream filename hangs
//     off, which is what a transparent proxy or a filename-preserving mirror
//     gives you: <mirror>/helm-v<version>-<goos>-<arch>.tar.gz
//  3. The official host.
//
// Only the URL differs; every path lands on the same checksum gate below.
func releaseURL(version, platform string) (string, error) {
	if exact := strings.TrimSpace(os.Getenv("FLOMATION_HELM_URL")); exact != "" {
		u, err := url.Parse(exact)
		if err != nil || u.Host == "" {
			return "", fmt.Errorf("FLOMATION_HELM_URL %q is not a valid URL", exact)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return "", fmt.Errorf("FLOMATION_HELM_URL must be an http(s) URL (got scheme %q)", u.Scheme)
		}
		return exact, nil
	}

	name := fmt.Sprintf("helm-v%s-%s.tar.gz", version, platform)
	if mirror := strings.TrimSpace(os.Getenv("FLOMATION_HELM_MIRROR")); mirror != "" {
		return strings.TrimRight(mirror, "/") + "/" + name, nil
	}
	return "https://get.helm.sh/" + name, nil
}

// expectedChecksum resolves the SHA-256 the downloaded archive must match:
// FLOMATION_HELM_SHA256 when the operator mirrors a version this file does not
// pin, otherwise the pinned value. An empty result means "never download".
func expectedChecksum(version, platform string) (string, error) {
	if override := strings.ToLower(strings.TrimSpace(os.Getenv("FLOMATION_HELM_SHA256"))); override != "" {
		if len(override) != 64 {
			return "", fmt.Errorf("FLOMATION_HELM_SHA256 must be a 64-character hex SHA-256 digest")
		}
		if _, err := hex.DecodeString(override); err != nil {
			return "", fmt.Errorf("FLOMATION_HELM_SHA256 is not valid hex: %w", err)
		}
		return override, nil
	}

	sums, known := pinnedChecksums[version]
	if !known {
		return "", fmt.Errorf("refusing to download helm %s: no checksum is pinned for it. "+
			"Install Helm on the runner, set binary_path, use the pinned version %s, "+
			"or declare the digest in FLOMATION_HELM_SHA256", version, DefaultVersion)
	}
	sum, known := sums[platform]
	if !known {
		return "", fmt.Errorf("refusing to download helm %s: no checksum is pinned for %s. "+
			"Install Helm on the runner, set binary_path, or declare the digest in FLOMATION_HELM_SHA256",
			version, platform)
	}
	return sum, nil
}

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
	want, err := expectedChecksum(version, platform)
	if err != nil {
		return "", err
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

// download fetches the artefact, checks it against the expected checksum, and
// installs the binary at dest. A mismatch never installs.
func download(ctx context.Context, version, platform, wantSum, dest string) error {
	src, err := releaseURL(version, platform)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp("", "helm-download-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	defer func() { _ = tmp.Close() }()

	gotSum, err := fetchToFile(ctx, src, tmp)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", src, err)
	}
	if !strings.EqualFold(gotSum, wantSum) {
		hint := ""
		if os.Getenv("FLOMATION_HELM_SHA256") == "" {
			// The likeliest cause when a mirror is in play: the operator is serving
			// a repackaged artefact, whose digest cannot match upstream's.
			hint = " — if this artefact was repackaged rather than mirrored byte-for-byte, " +
				"set FLOMATION_HELM_SHA256 to ITS sha256"
		}
		return fmt.Errorf("checksum mismatch for helm %s (%s) from %s: got %s, want %s; refusing to install%s",
			version, platform, src, gotSum, wantSum, hint)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return install(tmp.Name(), dest)
}

// install places the verified artefact at dest.
//
// Upstream ships a gzip tarball with the binary at <goos>-<arch>/helm, but an
// internal artefact host may just as well serve the naked executable. Which one
// arrived is decided by gzip's magic bytes rather than by the URL's extension,
// because a file named .tgz is not obliged to be one.
//
// Either way the bytes have already been checked against a checksum declared in
// advance, so this branch decides only how to unwrap them, never whether to
// trust them.
func install(artefact, dest string) error {
	f, err := os.Open(artefact) // #nosec G304 -- our own CreateTemp result
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	var magic [2]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		return fmt.Errorf("downloaded artefact is too small to be helm: %w", err)
	}
	if magic[0] == 0x1f && magic[1] == 0x8b {
		return extractBinary(artefact, dest)
	}
	return installRaw(artefact, dest)
}

// installRaw copies a naked executable into the cache, via temp-file-and-rename
// so a concurrent or aborted install can never leave a half-written binary for
// another process to execute.
func installRaw(artefact, dest string) error {
	src, err := os.Open(artefact) // #nosec G304 -- our own CreateTemp result
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()

	tmp, err := os.CreateTemp(filepath.Dir(dest), ".helm-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := io.Copy(tmp, io.LimitReader(src, maxBinarySize)); err != nil {
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

// fetchToFile streams url into f and returns the hex SHA-256 of what was written.
func fetchToFile(ctx context.Context, url string, f *os.File) (string, error) {
	client, err := downloadClient()
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
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
