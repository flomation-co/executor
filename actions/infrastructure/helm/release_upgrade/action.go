// Package infrastructure_helm_release_upgrade upgrades an existing Helm release
// to a new chart or new values — the shell-out equivalent of
// `helm upgrade <name> <chart> -o json`.
//
// Upgrade is destructive: it replaces the release's live resources and, unless
// --install is set, fails if the release does not already exist. It is therefore
// gated on confirm_destructive, which fails closed (see kubernetes.ConfirmDestructive).
//
// As with install, --atomic implies --wait and rolls the whole upgrade back on
// failure, and -o json is always passed so the release object parses — helm emits
// JSON even under --dry-run.
package infrastructure_helm_release_upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/helm"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Helm Upgrade Release"
	Description  = "Upgrade an existing Helm release to a new chart version or new values, optionally creating it if missing."
	Website      = "https://www.flomation.co"
	Icon         = "helm+arrow-up"
	Date         = "10/07/2026"
	Type         = core.ActionTypeAction
)

// defaultTimeoutSeconds mirrors helm's own default --timeout (5 minutes).
const defaultTimeoutSeconds = 300

var Inputs = [...]core.Connection{
	{Name: "api_server_url", Type: core.ConnectionTypeString, Label: "API Server URL", Placeholder: "https://your-cluster:6443 — the Kubernetes API endpoint", Required: true},
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{
		{Name: "Service Account Token", Value: "token"},
		{Name: "Client Certificate (mTLS)", Value: "client_cert"},
		{Name: "Kubeconfig (paste)", Value: "kubeconfig"},
	}},
	{Name: "service_account_token", Type: core.ConnectionTypeSecret, Label: "Service Account Token", Placeholder: "kubectl create token <serviceaccount> -n <namespace>", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "token"}}},
	{Name: "cluster_ca_cert", Type: core.ConnectionTypeText, Label: "Cluster CA Certificate (PEM)", Placeholder: "-----BEGIN CERTIFICATE----- … Leave blank to use the system trust store", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "token", "client_cert"}}},
	{Name: "client_certificate", Type: core.ConnectionTypeSecret, Label: "Client Certificate (PEM)", Placeholder: "-----BEGIN CERTIFICATE-----", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"client_cert"}}},
	{Name: "client_key", Type: core.ConnectionTypeSecret, Label: "Client Key (PEM)", Placeholder: "-----BEGIN RSA PRIVATE KEY-----", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"client_cert"}}},
	{Name: "kubeconfig", Type: core.ConnectionTypeSecret, Label: "Kubeconfig YAML", Placeholder: "Paste the full kubeconfig; the current-context is used", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"kubeconfig"}}},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure TLS", Placeholder: "Skip API server certificate verification — only for self-signed clusters with no CA to hand"},

	{Name: "helm_version", Type: core.ConnectionTypeString, Label: "Helm Version", Placeholder: "Leave blank to use the helm on the runner, or the pinned 3.21.3"},
	{Name: "binary_path", Type: core.ConnectionTypeString, Label: "Helm Binary Path", Placeholder: "/usr/local/bin/helm — overrides version lookup"},

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace the release lives in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Release Name", Placeholder: "The release to upgrade, e.g. my-nginx", Required: true},
	{Name: "chart", Type: core.ConnectionTypeString, Label: "Chart", Placeholder: "nginx, or oci://ghcr.io/o/chart, or https://.../chart.tgz", Required: true},
	{Name: "repo_url", Type: core.ConnectionTypeString, Label: "Repository URL", Placeholder: "https://charts.bitnami.com/bitnami — resolves a named chart without a repo add"},
	{Name: "chart_version", Type: core.ConnectionTypeString, Label: "Chart Version", Placeholder: "Pin a chart version, e.g. 15.5.2 (blank uses the latest)"},
	{Name: "repo_username", Type: core.ConnectionTypeString, Label: "Repository Username", Placeholder: "For a private chart repository (Nexus, Artifactory, an OCI registry)"},
	{Name: "repo_password", Type: core.ConnectionTypeSecret, Label: "Repository Password", Placeholder: "Never passed on the command line — written to a 0600 file, or piped to registry login"},
	{Name: "repo_ca_cert", Type: core.ConnectionTypeText, Label: "Repository CA Certificate (PEM)", Placeholder: "-----BEGIN CERTIFICATE----- … for a repository behind an internal CA"},
	{Name: "repo_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure Repository TLS", Placeholder: "Skip certificate verification when fetching the chart — only for a self-signed internal repository"},
	{Name: "values", Type: core.ConnectionTypeCode, Label: "Values (YAML)", Placeholder: "YAML overriding the chart defaults, e.g. replicaCount: 2"},
	{Name: "create_namespace", Type: core.ConnectionTypeBoolean, Label: "Create Namespace", Placeholder: "Create the target namespace if it does not already exist"},
	{Name: "wait", Type: core.ConnectionTypeBoolean, Label: "Wait for Ready", Placeholder: "Block until every resource is ready, or the timeout elapses"},
	{Name: "timeout_seconds", Type: core.ConnectionTypeInteger, Label: "Timeout (seconds)", Placeholder: "How long to wait for readiness (default 300)"},
	{Name: "atomic", Type: core.ConnectionTypeBoolean, Label: "Atomic", Placeholder: "Roll back automatically if the upgrade fails"},
	{Name: "dry_run", Type: core.ConnectionTypeBoolean, Label: "Dry Run", Placeholder: "Render and validate the upgrade without applying anything"},
	{Name: "install_if_missing", Type: core.ConnectionTypeBoolean, Label: "Install if Missing", Placeholder: "Create the release when it does not exist (helm upgrade --install)"},
	{Name: "reuse_values", Type: core.ConnectionTypeBoolean, Label: "Reuse Values", Placeholder: "Reuse the previous release's values, merging any new ones on top"},
	{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Label: "Confirm Destructive Action", Placeholder: "This permanently changes cluster state. Tick to allow, or bind a variable such as ${var.approved}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Release Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Release"},
	{Name: "revision", Type: core.ConnectionTypeInteger, Label: "Revision"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := kubernetes.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	namespace, err := kubernetes.RequiredString("namespace", inputs)
	if err != nil {
		return nil, err
	}
	name, err := kubernetes.RequiredString("name", inputs)
	if err != nil {
		return nil, err
	}
	chart, err := kubernetes.RequiredString("chart", inputs)
	if err != nil {
		return nil, err
	}
	if err := kubernetes.ConfirmDestructive(inputs, "upgrade the release "+name); err != nil {
		return helm.ErrorResult(err.Error()), nil
	}

	repoURL := kubernetes.OptionalString("repo_url", inputs)
	chartVersion := kubernetes.OptionalString("chart_version", inputs)
	repoUsername := kubernetes.OptionalString("repo_username", inputs)
	repoPassword := kubernetes.OptionalString("repo_password", inputs)
	repoCACert := kubernetes.OptionalString("repo_ca_cert", inputs)
	repoInsecure := kubernetes.BoolInput("repo_insecure", inputs)

	source := helm.ChartSource{
		Chart:        chart,
		RepoURL:      repoURL,
		ChartVersion: chartVersion,
		Username:     repoUsername,
		Password:     repoPassword,
		CACert:       repoCACert,
		Insecure:     repoInsecure,
	}
	valuesYAML := kubernetes.OptionalString("values", inputs)
	createNamespace := kubernetes.BoolInput("create_namespace", inputs)
	wait := kubernetes.BoolInput("wait", inputs)
	atomic := kubernetes.BoolInput("atomic", inputs)
	dryRun := kubernetes.BoolInput("dry_run", inputs)
	installIfMissing := kubernetes.BoolInput("install_if_missing", inputs)
	reuseValues := kubernetes.BoolInput("reuse_values", inputs)

	timeoutSec := defaultTimeoutSeconds
	if n, ok := kubernetes.OptionalInt("timeout_seconds", inputs); ok && n > 0 {
		timeoutSec = n
	}

	version := kubernetes.OptionalString("helm_version", inputs)
	binaryPath := kubernetes.OptionalString("binary_path", inputs)

	// Invoke applies its own 15-minute timeout, so a bare Background context is
	// correct here — a --wait upgrade is legitimately long-running.
	res, err := helm.WithSession(context.Background(), auth, version, binaryPath, namespace, valuesYAML, func(s *helm.Session) (*helm.RunResult, error) {
		chartArgs, err := s.ResolveChart(source)
		if err != nil {
			return nil, err
		}
		args := append([]string{"upgrade", name}, chartArgs...)
		args = append(args, "-n", namespace)
		args = append(args, s.ValuesArgs...)
		if installIfMissing {
			args = append(args, "--install")
			if createNamespace {
				args = append(args, "--create-namespace")
			}
		}
		if reuseValues {
			args = append(args, "--reuse-values")
		}
		if wait {
			args = append(args, "--wait")
		}
		args = append(args, "--timeout", strconv.Itoa(timeoutSec)+"s")
		if atomic {
			args = append(args, "--atomic")
		}
		if dryRun {
			args = append(args, "--dry-run")
		}
		return s.Run(append(args, "-o", "json")...)
	})
	if err != nil {
		return helm.ErrorResult(err.Error()), nil
	}
	if res.Failed() {
		return helm.ErrorResult(res.Message()), nil
	}

	rel := parseReleaseJSON(res.Stdout)
	revision, status := releaseRevisionStatus(rel)

	summary := fmt.Sprintf("Upgraded release %s to revision %d (%s)", name, revision, status)
	if dryRun {
		summary = fmt.Sprintf("Dry run for release %s completed — nothing was applied", name)
	}

	out := helm.ReleaseResult(name, rel, summary)
	out["revision"] = revision
	out["status"] = status
	return out, nil
}

// parseReleaseJSON decodes the release object helm prints under -o json. A blank
// or unparseable body yields nil rather than an error: helm exited zero, so the
// operation succeeded even if the report is not the shape we expected.
func parseReleaseJSON(stdout string) map[string]interface{} {
	s := strings.TrimSpace(stdout)
	if s == "" {
		return nil
	}
	var rel map[string]interface{}
	if err := json.Unmarshal([]byte(s), &rel); err != nil {
		return nil
	}
	return rel
}

// releaseRevisionStatus pulls the revision (release.version) and lifecycle status
// (release.info.status) out of a decoded release object.
func releaseRevisionStatus(rel map[string]interface{}) (int, string) {
	revision, status := 0, ""
	if rel == nil {
		return revision, status
	}
	if v, ok := rel["version"].(float64); ok {
		revision = int(v)
	}
	if info, ok := rel["info"].(map[string]interface{}); ok {
		if s, ok := info["status"].(string); ok {
			status = s
		}
	}
	return revision, status
}
