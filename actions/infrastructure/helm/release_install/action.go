// Package infrastructure_helm_release_install installs a Helm chart as a new
// release — the shell-out equivalent of `helm install <name> <chart> -o json`.
//
// Unlike upgrade, install is not gated on confirm_destructive: it only ever
// creates. Helm refuses (rather than overwrites) when a release of the same name
// already exists in the namespace, so the worst outcome is a no-op error, never a
// clobbered release.
//
// Two flag notes the code cannot state: --atomic implies --wait — helm rolls the
// install back automatically if the release does not become ready before the
// timeout — and -o json is always passed so the release object parses, since helm
// emits JSON even under --dry-run.
package infrastructure_helm_release_install

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
	Name         = "Helm Install Release"
	Description  = "Install a Helm chart as a new release on a Kubernetes cluster, optionally waiting for it to become ready."
	Website      = "https://www.flomation.co"
	Icon         = "helm+plus"
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

	{Name: "helm_version", Type: core.ConnectionTypeString, Label: "Helm Version", Placeholder: "Leave blank to use the helm on the runner"},
	{Name: "binary_path", Type: core.ConnectionTypeString, Label: "Helm Binary Path", Placeholder: "/usr/local/bin/helm — overrides version lookup"},

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace to install the release into", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Release Name", Placeholder: "A name for this release, e.g. my-nginx", Required: true},
	{Name: "chart", Type: core.ConnectionTypeString, Label: "Chart", Placeholder: "nginx, or oci://ghcr.io/o/chart, or https://.../chart.tgz", Required: true},
	{Name: "repo_url", Type: core.ConnectionTypeString, Label: "Repository URL", Placeholder: "https://charts.bitnami.com/bitnami — resolves a named chart without a repo add"},
	{Name: "chart_version", Type: core.ConnectionTypeString, Label: "Chart Version", Placeholder: "Pin a chart version, e.g. 15.5.2 (blank installs the latest)"},
	{Name: "values", Type: core.ConnectionTypeCode, Label: "Values (YAML)", Placeholder: "YAML overriding the chart defaults, e.g. replicaCount: 2"},
	{Name: "create_namespace", Type: core.ConnectionTypeBoolean, Label: "Create Namespace", Placeholder: "Create the target namespace if it does not already exist"},
	{Name: "wait", Type: core.ConnectionTypeBoolean, Label: "Wait for Ready", Placeholder: "Block until every resource is ready, or the timeout elapses"},
	{Name: "timeout_seconds", Type: core.ConnectionTypeInteger, Label: "Timeout (seconds)", Placeholder: "How long to wait for readiness (default 300)"},
	{Name: "atomic", Type: core.ConnectionTypeBoolean, Label: "Atomic", Placeholder: "Roll back automatically if the install fails"},
	{Name: "dry_run", Type: core.ConnectionTypeBoolean, Label: "Dry Run", Placeholder: "Render and validate the release without applying anything"},
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

	repoURL := kubernetes.OptionalString("repo_url", inputs)
	chartVersion := kubernetes.OptionalString("chart_version", inputs)
	valuesYAML := kubernetes.OptionalString("values", inputs)
	createNamespace := kubernetes.BoolInput("create_namespace", inputs)
	wait := kubernetes.BoolInput("wait", inputs)
	atomic := kubernetes.BoolInput("atomic", inputs)
	dryRun := kubernetes.BoolInput("dry_run", inputs)

	timeoutSec := defaultTimeoutSeconds
	if n, ok := kubernetes.OptionalInt("timeout_seconds", inputs); ok && n > 0 {
		timeoutSec = n
	}

	version := kubernetes.OptionalString("helm_version", inputs)
	binaryPath := kubernetes.OptionalString("binary_path", inputs)

	// Invoke applies its own 15-minute timeout, so a bare Background context is
	// correct here — a --wait install is legitimately long-running.
	res, err := helm.InvokeWithValues(context.Background(), auth, version, binaryPath, namespace, valuesYAML, func(valuesArgs []string) []string {
		args := []string{"install", name}
		args = helm.AddChartSource(args, chart, repoURL, chartVersion)
		args = append(args, "-n", namespace)
		args = append(args, valuesArgs...)
		if createNamespace {
			args = append(args, "--create-namespace")
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
		return append(args, "-o", "json")
	})
	if err != nil {
		return helm.ErrorResult(err.Error()), nil
	}
	if res.Failed() {
		return helm.ErrorResult(res.Message()), nil
	}

	rel := parseReleaseJSON(res.Stdout)
	revision, status := releaseRevisionStatus(rel)

	summary := fmt.Sprintf("Installed release %s (revision %d, %s)", name, revision, status)
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
