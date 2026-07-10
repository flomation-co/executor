// Package helm holds the shared runtime for the infrastructure/helm/* actions:
// resolving a helm binary (binary.go), running it against a synthesised
// kubeconfig in a throwaway HELM_* home (this file), and reading release state
// straight out of the cluster's Helm storage Secrets (release.go).
//
// Why the split. Helm has no server-side API — Tiller is long gone, and the
// modern client does all its work locally, storing the result as a Secret in the
// release's namespace. So:
//
//   - READS (list, get, status, history) decode those Secrets over the same
//     Kubernetes REST client the sibling kubernetes package uses. They need no
//     binary at all, which means they work on any runner, and they cost one API
//     call instead of a process spawn.
//   - WRITES (install, upgrade, rollback, uninstall, test) and the local render
//     commands (template, lint, show) need Helm's chart engine, so they shell out
//     to the binary — the same shape as the OpenTofu actions driving `tofu`.
//
// Credentials never reach the process argument list. helm exposes --kube-token,
// but argv is world-readable through /proc on Linux, so a bearer token passed
// that way is visible to every other process on the runner for the lifetime of
// the command. Instead every invocation writes a 0600 kubeconfig into a
// per-invocation temp directory and passes --kubeconfig; the directory is removed
// when the action returns.
package helm

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	yaml "go.yaml.in/yaml/v3"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

// commandTimeout bounds a single helm invocation. An install that waits for
// pods to become ready is legitimately slow, so this is generous; actions that
// pass --wait should keep their own --timeout below it.
const commandTimeout = 15 * time.Minute

// RunResult is the outcome of one helm invocation. A non-zero ExitCode is
// returned as data rather than as a Go error, so a caller can surface helm's own
// stderr — which is almost always the useful message — instead of "exit status 1".
type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Failed reports whether helm exited non-zero.
func (r *RunResult) Failed() bool { return r != nil && r.ExitCode != 0 }

// Message renders helm's own diagnostic, preferring stderr (where helm writes
// errors) and falling back to stdout.
func (r *RunResult) Message() string {
	if r == nil {
		return ""
	}
	if s := strings.TrimSpace(r.Stderr); s != "" {
		return s
	}
	return strings.TrimSpace(r.Stdout)
}

// BinaryInputs are the two inputs every binary-backed helm action carries, so an
// operator can pin a version or point at a pre-installed binary.
var BinaryInputs = []core.Connection{
	{
		Name:        "helm_version",
		Type:        core.ConnectionTypeString,
		Label:       "Helm Version",
		Placeholder: "Leave blank to use the helm on the runner, or the pinned " + DefaultVersion,
	},
	{
		Name:        "binary_path",
		Type:        core.ConnectionTypeString,
		Label:       "Helm Binary Path",
		Placeholder: "/usr/local/bin/helm — overrides version lookup",
	},
}

// kubeconfigFor renders an Auth as a minimal kubeconfig.
//
// certificate-authority-data and insecure-skip-tls-verify are mutually exclusive
// — client-go rejects a config carrying both with "specifying a root certificates
// file with the insecure flag is not allowed" — so the insecure opt-in drops the
// CA rather than sitting alongside it.
func kubeconfigFor(a kubernetes.Auth, namespace string) ([]byte, error) {
	cluster := map[string]interface{}{"server": a.Server}
	if a.Insecure {
		cluster["insecure-skip-tls-verify"] = true
	} else if len(a.CACert) > 0 {
		cluster["certificate-authority-data"] = base64.StdEncoding.EncodeToString(a.CACert)
	}

	user := map[string]interface{}{}
	switch {
	case a.Token != "":
		user["token"] = a.Token
	case len(a.ClientCert) > 0 && len(a.ClientKey) > 0:
		user["client-certificate-data"] = base64.StdEncoding.EncodeToString(a.ClientCert)
		user["client-key-data"] = base64.StdEncoding.EncodeToString(a.ClientKey)
	default:
		return nil, fmt.Errorf("no usable Kubernetes credential: supply a service account token or a client certificate")
	}

	ctx := map[string]interface{}{"cluster": "flomation", "user": "flomation"}
	if ns := strings.TrimSpace(namespace); ns != "" {
		ctx["namespace"] = ns
	}

	return yaml.Marshal(map[string]interface{}{
		"apiVersion":      "v1",
		"kind":            "Config",
		"current-context": "flomation",
		"clusters":        []interface{}{map[string]interface{}{"name": "flomation", "cluster": cluster}},
		"users":           []interface{}{map[string]interface{}{"name": "flomation", "user": user}},
		"contexts":        []interface{}{map[string]interface{}{"name": "flomation", "context": ctx}},
	})
}

// Session is a prepared helm working directory: a resolved binary, a 0600
// kubeconfig, and — when the action supplied values — a values file. Several
// commands can run against it, which is what `helm lint` needs: lint takes a
// chart *directory*, so a remote chart must be pulled and unpacked first, and
// both commands have to see the same temp home.
type Session struct {
	// Home is the temp directory every command runs in. It is removed when the
	// session ends; nothing written here outlives the action.
	Home string
	// ValuesArgs is the "-f <path>" pair for the supplied values, or nil.
	ValuesArgs []string

	ctx        context.Context
	bin        string
	kubeconfig string
	auth       kubernetes.Auth
}

// Run executes one helm command in the session's home.
func (s *Session) Run(args ...string) (*RunResult, error) {
	full := append([]string{"--kubeconfig", s.kubeconfig}, args...)
	cmd := exec.CommandContext(s.ctx, s.bin, full...) // #nosec G204 -- bin comes from EnsureBinary; args are built by the actions, never a raw operator string
	cmd.Dir = s.Home
	cmd.Env = buildEnv(s.Home)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// Give a cancelled process a moment to exit before its pipes are force-closed,
	// mirroring the OpenTofu actions.
	cmd.WaitDelay = 5 * time.Second

	res := &RunResult{}
	runErr := cmd.Run()
	res.Stdout = stdout.String()
	res.Stderr = kubernetes.Redact(s.auth, stderr.String())

	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			res.ExitCode = exitErr.ExitCode()
			return res, nil // a non-zero exit is data, not a failure to run
		}
		if s.ctx.Err() != nil {
			return res, fmt.Errorf("helm timed out after %s", commandTimeout)
		}
		return res, fmt.Errorf("could not run helm: %w", runErr)
	}
	return res, nil
}

// WithSession prepares a working directory, hands it to fn, and tears it down.
//
// The directory holds the kubeconfig, so credentials never reach the process
// argument list — helm exposes --kube-token, but argv is world-readable through
// /proc, and a bearer token there is visible to every other process on the runner
// for the lifetime of the command. The directory is removed on every path.
//
// namespace scopes the kubeconfig's default context. Callers still pass an
// explicit -n; the context default only matters for subcommands that read it
// implicitly.
func WithSession(ctx context.Context, a kubernetes.Auth, version, binaryPath, namespace, valuesYAML string, fn func(*Session) (*RunResult, error)) (*RunResult, error) {
	bin, err := EnsureBinary(ctx, version, binaryPath)
	if err != nil {
		return nil, err
	}

	home, err := os.MkdirTemp("", "flomation-helm-")
	if err != nil {
		return nil, fmt.Errorf("could not create a working directory for helm: %w", err)
	}
	defer func() { _ = os.RemoveAll(home) }()

	kubeconfig, err := kubeconfigFor(a, namespace)
	if err != nil {
		return nil, err
	}
	kubeconfigPath := home + "/kubeconfig"
	if err := os.WriteFile(kubeconfigPath, kubeconfig, 0o600); err != nil {
		return nil, fmt.Errorf("could not write kubeconfig: %w", err)
	}

	valuesArgs, err := WriteValuesFile(home, valuesYAML)
	if err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	return fn(&Session{
		Home:       home,
		ValuesArgs: valuesArgs,
		ctx:        runCtx,
		bin:        bin,
		kubeconfig: kubeconfigPath,
		auth:       a,
	})
}

// SoleChartDir returns the single directory `helm pull --untar --untardir` left
// behind. The extracted directory is named after the chart, which the caller
// cannot know in advance for an oci:// or .tgz reference, so it is discovered.
func SoleChartDir(untarDir string) (string, error) {
	entries, err := os.ReadDir(untarDir)
	if err != nil {
		return "", fmt.Errorf("chart was not unpacked: %w", err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) != 1 {
		return "", fmt.Errorf("expected exactly one unpacked chart directory, found %d", len(dirs))
	}
	return untarDir + "/" + dirs[0], nil
}

// Invoke runs a single helm command against the supplied connection.
func Invoke(ctx context.Context, a kubernetes.Auth, version, binaryPath, namespace string, args []string) (*RunResult, error) {
	return WithSession(ctx, a, version, binaryPath, namespace, "", func(s *Session) (*RunResult, error) {
		return s.Run(args...)
	})
}

// buildEnv gives helm a self-contained, writable home. Helm 3 reads HELM_* first
// and falls back to the XDG variables, so both are set — a host that ships an
// older client still lands inside the temp directory rather than in the runner's
// real $HOME.
//
// PATH is inherited because helm shells out to plugins and to git for some chart
// sources. Nothing else is: a chart's values must not be able to read the
// runner's environment.
func buildEnv(home string) []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"HELM_CACHE_HOME=" + home + "/cache",
		"HELM_CONFIG_HOME=" + home + "/config",
		"HELM_DATA_HOME=" + home + "/data",
		"XDG_CACHE_HOME=" + home + "/cache",
		"XDG_CONFIG_HOME=" + home + "/config",
		"XDG_DATA_HOME=" + home + "/data",
		"HELM_EXPERIMENTAL_OCI=1",
		"LANG=en_GB.UTF-8",
		"TERM=dumb",
	}
}

// ---------------------------------------------------------------------------
// Argument helpers
// ---------------------------------------------------------------------------

// AddChartSource appends the chart reference and, when the operator supplied a
// repository URL, the --repo flag that resolves it without a `helm repo add`.
//
// This is what keeps the actions stateless. `helm repo add` writes a
// repositories.yaml into the Helm config home; on a one-shot executor that home
// is a temp directory discarded when the action returns, so a repo added by one
// action would not exist for the next. Passing --repo (or an oci:// reference,
// or a full https:// chart URL) resolves the chart in a single invocation, with
// nothing to persist.
func AddChartSource(args []string, chart, repoURL, chartVersion string) []string {
	args = append(args, chart)
	if repoURL != "" {
		args = append(args, "--repo", repoURL)
	}
	if chartVersion != "" {
		args = append(args, "--version", chartVersion)
	}
	return args
}

// WriteValuesFile writes operator-supplied YAML/JSON values into the invocation's
// working directory and returns the -f arguments for it. Values are passed by
// file rather than by --set so that a value containing a comma, a dot, or a brace
// survives — --set has its own escaping grammar and silently mangles anything
// that collides with it.
//
// An empty body yields no arguments.
func WriteValuesFile(dir, valuesYAML string) ([]string, error) {
	body := strings.TrimSpace(valuesYAML)
	if body == "" {
		return nil, nil
	}
	// Reject a values document that isn't a mapping before helm does, since its
	// own error ("cannot unmarshal !!seq into map") reads like a bug in Flomation.
	var probe interface{}
	if err := yaml.Unmarshal([]byte(body), &probe); err != nil {
		return nil, fmt.Errorf("values must be valid YAML or JSON: %w", err)
	}
	if probe != nil {
		if _, ok := probe.(map[string]interface{}); !ok {
			return nil, fmt.Errorf("values must be a mapping of keys to values, e.g. replicaCount: 2")
		}
	}

	path := dir + "/values.yaml"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return nil, fmt.Errorf("could not write values file: %w", err)
	}
	return []string{"-f", path}, nil
}

// InvokeWithValues is Invoke plus an optional values document, written into the
// invocation's temp directory so it lives exactly as long as the process does.
//
// buildArgs receives the "-f <path>" pair for the values file (nil when no values
// were supplied) and returns the full argument list.
func InvokeWithValues(ctx context.Context, a kubernetes.Auth, version, binaryPath, namespace, valuesYAML string, buildArgs func(valuesArgs []string) []string) (*RunResult, error) {
	return WithSession(ctx, a, version, binaryPath, namespace, valuesYAML, func(s *Session) (*RunResult, error) {
		return s.Run(buildArgs(s.ValuesArgs)...)
	})
}

// ---------------------------------------------------------------------------
// Result shaping
// ---------------------------------------------------------------------------

// ErrorResult is the standard soft-failure output map, returned alongside a nil
// error so the engine records it on the error port rather than aborting the flow.
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// ReleaseResult shapes a single-release response into the standard output map.
func ReleaseResult(name string, obj interface{}, summary string) map[string]interface{} {
	return map[string]interface{}{
		"id":          name,
		"result":      obj,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// ListResult shapes a collection of releases.
func ListResult(items []interface{}, summary string) map[string]interface{} {
	if items == nil {
		items = []interface{}{}
	}
	return map[string]interface{}{
		"results":     items,
		"count":       len(items),
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}
