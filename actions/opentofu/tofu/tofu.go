// Package tofu provides the shared runtime plumbing for the OpenTofu actions:
// locating (or downloading) a pinned `tofu` binary, building an automation-safe
// environment, running commands, and parsing OpenTofu's machine-readable output.
//
// It deliberately holds no executor types so it stays decoupled from the action
// layer — callers pass plain maps and receive plain results.
//
// Binary resolution order (see EnsureBinary):
//  1. An explicit binary path supplied by the action.
//  2. A previously-downloaded binary in the cache for the requested version.
//  3. A fresh download of the pinned release, verified against the official
//     SHA256SUMS file published alongside the release.
//  4. As a last resort, any `tofu` already on PATH (e.g. installed by a DEB/RPM
//     package), so the actions also work where the host provides the binary.
package tofu

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

// DefaultVersion is the OpenTofu release the actions pin to when the workflow
// does not request a specific version. Bump deliberately; downloads are
// checksum-verified against this exact tag.
const DefaultVersion = "1.9.1"

const binaryName = "tofu"

// downloadMu serialises downloads within a single process so concurrent flows
// resolving the same version don't race on the cache. Cross-process safety
// relies on the atomic rename at the end of extractBinary.
var downloadMu sync.Mutex

// httpClient bounds network operations; downloads of a few tens of MB over a
// slow link should still comfortably finish inside this.
var httpClient = &http.Client{Timeout: 5 * time.Minute}

// EnsureBinary returns a path to a usable `tofu` binary, downloading and caching
// the pinned release if necessary. override, when non-empty, must point at an
// existing binary and short-circuits all resolution.
func EnsureBinary(ctx context.Context, version, override string) (string, error) {
	if override != "" {
		if _, err := os.Stat(override); err != nil {
			return "", fmt.Errorf("opentofu binary_path %q is not usable: %w", override, err)
		}
		return override, nil
	}

	if version == "" {
		version = DefaultVersion
	}

	dest := cachePath(version)
	if isExecutableFile(dest) {
		return dest, nil
	}

	downloadMu.Lock()
	defer downloadMu.Unlock()

	// Re-check after acquiring the lock: another goroutine may have just
	// finished downloading the same version.
	if isExecutableFile(dest) {
		return dest, nil
	}

	if err := download(ctx, version, dest); err != nil {
		// Fall back to a host-provided tofu (package install) before giving up,
		// so air-gapped or download-restricted deployments still work.
		if p, lookErr := exec.LookPath(binaryName); lookErr == nil {
			return p, nil
		}
		return "", fmt.Errorf("could not obtain tofu %s (and none on PATH): %w", version, err)
	}

	return dest, nil
}

// RunConfig captures everything needed to prepare an OpenTofu working directory.
type RunConfig struct {
	WorkDir       string
	Version       string
	BinaryPath    string
	TFVars        map[string]string // exported as TF_VAR_<key>
	ExtraEnv      map[string]string // raw env (e.g. provider credentials)
	BackendConfig map[string]string // passed as -backend-config to `tofu init`
}

// RunResult is the outcome of a single tofu invocation. A non-zero ExitCode is
// reported here rather than as an error so callers can branch on it; err is only
// non-nil for failures to start/run the process itself (or a timeout).
type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Prepare resolves the binary, builds the environment, and runs `tofu init`
// (wiring in any backend config). It returns the binary path and environment so
// the caller can run the subsequent plan/apply/destroy command.
func Prepare(ctx context.Context, c RunConfig) (bin string, env []string, init *RunResult, err error) {
	bin, err = EnsureBinary(ctx, c.Version, c.BinaryPath)
	if err != nil {
		return "", nil, nil, err
	}

	env = BuildEnv(c.WorkDir, c.TFVars, c.ExtraEnv)

	args := []string{"init", "-input=false", "-no-color"}
	for k, v := range c.BackendConfig {
		args = append(args, "-backend-config="+k+"="+v)
	}

	init, err = Run(ctx, bin, c.WorkDir, env, args...)
	return bin, env, init, err
}

// Run executes tofu with the given args in workDir. Stdout/stderr are captured.
func Run(ctx context.Context, bin, workDir string, env []string, args ...string) (*RunResult, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = workDir
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// Give a cancelled/timed-out process a moment to exit before the pipes are
	// force-closed, mirroring the other subprocess actions.
	cmd.WaitDelay = 5 * time.Second

	res := &RunResult{}
	err := cmd.Run()
	res.Stdout = stdout.String()
	res.Stderr = stderr.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			res.ExitCode = exitErr.ExitCode()
			return res, nil // non-zero exit is data, not a hard failure
		}
		return res, err
	}
	return res, nil
}

// BuildEnv constructs an automation-friendly environment. home must be writable
// (OpenTofu writes plugin/state metadata under it). TF_VAR_* are derived from
// tfVars; extraEnv is merged verbatim for provider credentials and the like.
func BuildEnv(home string, tfVars, extraEnv map[string]string) []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"TF_IN_AUTOMATION=1",
		"TF_INPUT=0",
		"LANG=en_GB.UTF-8",
		"TERM=dumb",
	}

	if pc := pluginCacheDir(); pc != "" {
		if err := os.MkdirAll(pc, 0o755); err == nil {
			env = append(env, "TF_PLUGIN_CACHE_DIR="+pc)
		}
	}

	for k, v := range tfVars {
		if k == "" {
			continue
		}
		env = append(env, "TF_VAR_"+k+"="+v)
	}
	for k, v := range extraEnv {
		if k == "" {
			continue
		}
		env = append(env, k+"="+v)
	}
	return env
}

// ChangeSummary holds the add/change/destroy counts OpenTofu reports.
type ChangeSummary struct {
	Add     int
	Change  int
	Destroy int
}

// ParsePlanSummary scans `tofu plan -json` / `tofu apply -json` newline-delimited
// output for the change_summary message and returns the counts. found is false
// when no summary line was present (e.g. an error before planning completed).
func ParsePlanSummary(jsonStream string) (summary ChangeSummary, found bool) {
	sc := bufio.NewScanner(strings.NewReader(jsonStream))
	// Plan output can contain long single lines (full resource diffs); raise the
	// scanner ceiling well above the default 64 KiB.
	sc.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)

	for sc.Scan() {
		var line struct {
			Type    string `json:"type"`
			Changes struct {
				Add    int `json:"add"`
				Change int `json:"change"`
				Remove int `json:"remove"`
			} `json:"changes"`
		}
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			continue // non-JSON or unrelated line
		}
		if line.Type == "change_summary" {
			summary = ChangeSummary{
				Add:     line.Changes.Add,
				Change:  line.Changes.Change,
				Destroy: line.Changes.Remove,
			}
			found = true
		}
	}
	return summary, found
}

func (s ChangeSummary) HasChanges() bool { return s.Add+s.Change+s.Destroy > 0 }

func (s ChangeSummary) String() string {
	return fmt.Sprintf("+%d ~%d -%d", s.Add, s.Change, s.Destroy)
}

// --- download / cache internals ---------------------------------------------

func cacheRoot() string {
	if d := os.Getenv("FLOMATION_TOFU_CACHE"); d != "" {
		return d
	}
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "flomation", "opentofu")
}

func cachePath(version string) string {
	return filepath.Join(cacheRoot(), version, binaryName)
}

func pluginCacheDir() string {
	return filepath.Join(cacheRoot(), "plugin-cache")
}

func isExecutableFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular() && fi.Mode()&0o111 != 0
}

func download(ctx context.Context, version, dest string) error {
	goos, arch := runtime.GOOS, runtime.GOARCH
	zipName := fmt.Sprintf("tofu_%s_%s_%s.zip", version, goos, arch)
	base := fmt.Sprintf("https://github.com/opentofu/opentofu/releases/download/v%s", version)
	zipURL := base + "/" + zipName
	sumsURL := fmt.Sprintf("%s/tofu_%s_SHA256SUMS", base, version)

	wantSum, err := fetchChecksum(ctx, sumsURL, zipName)
	if err != nil {
		return fmt.Errorf("resolving checksum: %w", err)
	}

	tmpZip, err := os.CreateTemp("", "tofu-download-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmpZip.Name())
	defer tmpZip.Close()

	gotSum, err := fetchToFile(ctx, zipURL, tmpZip)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", zipName, err)
	}
	if !strings.EqualFold(gotSum, wantSum) {
		// TODO(opentofu): also verify the cosign/GPG signature of SHA256SUMS for
		// defence against a compromised release asset, not just integrity.
		return fmt.Errorf("checksum mismatch for %s: got %s, want %s", zipName, gotSum, wantSum)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return extractBinary(tmpZip.Name(), dest)
}

func fetchChecksum(ctx context.Context, url, wantFile string) (string, error) {
	body, err := httpGet(ctx, url)
	if err != nil {
		return "", err
	}
	defer body.Close()

	sc := bufio.NewScanner(body)
	for sc.Scan() {
		// Lines look like: "<sha256>  tofu_1.9.1_linux_amd64.zip"
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && fields[1] == wantFile {
			return fields[0], nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("no checksum entry for %s", wantFile)
}

// fetchToFile streams url into f and returns the hex-encoded SHA-256 of the body.
func fetchToFile(ctx context.Context, url string, f *os.File) (string, error) {
	body, err := httpGet(ctx, url)
	if err != nil {
		return "", err
	}
	defer body.Close()

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), body); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func httpGet(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: unexpected status %s", url, resp.Status)
	}
	return resp.Body, nil
}

// extractBinary pulls the `tofu` entry out of the release zip and installs it at
// dest via a temp-file-and-rename so concurrent/aborted extractions can't leave
// a half-written binary in the cache.
func extractBinary(zipPath, dest string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, zf := range r.File {
		if filepath.Base(zf.Name) != binaryName {
			continue
		}

		src, err := zf.Open()
		if err != nil {
			return err
		}
		defer src.Close()

		tmp, err := os.CreateTemp(filepath.Dir(dest), ".tofu-*")
		if err != nil {
			return err
		}
		tmpName := tmp.Name()

		if _, err := io.Copy(tmp, src); err != nil {
			tmp.Close()
			os.Remove(tmpName)
			return err
		}
		if err := tmp.Chmod(0o755); err != nil {
			tmp.Close()
			os.Remove(tmpName)
			return err
		}
		if err := tmp.Close(); err != nil {
			os.Remove(tmpName)
			return err
		}
		return os.Rename(tmpName, dest)
	}

	return fmt.Errorf("%s binary not found inside release archive", binaryName)
}
