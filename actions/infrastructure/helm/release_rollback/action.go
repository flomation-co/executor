// Package infrastructure_helm_release_rollback rolls a Helm release back to an
// earlier revision — the shell-out equivalent of `helm rollback <name> [revision]`.
//
// Rollback is destructive: it re-applies an old revision's resources over the
// live ones and supersedes the current revision, so it is gated on
// confirm_destructive, which fails closed (see kubernetes.ConfirmDestructive).
//
// A blank or zero revision rolls back to the immediately previous revision.
// Unlike install/upgrade, `helm rollback` prints a plain-text confirmation, not
// JSON — so no -o json is passed, and its stdout is surfaced verbatim.
package infrastructure_helm_release_rollback

import (
	"context"
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
	Name         = "Helm Rollback Release"
	Description  = "Roll a Helm release back to a previous revision. Requires explicit confirmation."
	Website      = "https://www.flomation.co"
	Icon         = "helm+rotate-left"
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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace the release lives in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Release Name", Placeholder: "The release to roll back, e.g. my-nginx", Required: true},
	{Name: "revision", Type: core.ConnectionTypeInteger, Label: "Revision", Placeholder: "0 or blank rolls back to the previous revision"},
	{Name: "wait", Type: core.ConnectionTypeBoolean, Label: "Wait for Ready", Placeholder: "Block until every resource is ready, or the timeout elapses"},
	{Name: "timeout_seconds", Type: core.ConnectionTypeInteger, Label: "Timeout (seconds)", Placeholder: "How long to wait for readiness (default 300)"},
	{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Label: "Confirm Destructive Action", Placeholder: "This permanently changes cluster state. Tick to allow, or bind a variable such as ${var.approved}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Release Name"},
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

	revision := 0
	if n, ok := kubernetes.OptionalInt("revision", inputs); ok && n > 0 {
		revision = n
	}

	target := "the previous revision"
	if revision > 0 {
		target = fmt.Sprintf("revision %d", revision)
	}
	if err := kubernetes.ConfirmDestructive(inputs, "roll back the release "+name+" to "+target); err != nil {
		return helm.ErrorResult(err.Error()), nil
	}

	wait := kubernetes.BoolInput("wait", inputs)
	timeoutSec := defaultTimeoutSeconds
	if n, ok := kubernetes.OptionalInt("timeout_seconds", inputs); ok && n > 0 {
		timeoutSec = n
	}

	version := kubernetes.OptionalString("helm_version", inputs)
	binaryPath := kubernetes.OptionalString("binary_path", inputs)

	args := []string{"rollback", name}
	if revision > 0 {
		args = append(args, strconv.Itoa(revision))
	}
	args = append(args, "-n", namespace)
	if wait {
		args = append(args, "--wait")
	}
	args = append(args, "--timeout", strconv.Itoa(timeoutSec)+"s")

	// Invoke applies its own 15-minute timeout, so a bare Background context is
	// correct here — a --wait rollback is legitimately long-running.
	res, err := helm.Invoke(context.Background(), auth, version, binaryPath, namespace, args)
	if err != nil {
		return helm.ErrorResult(err.Error()), nil
	}
	if res.Failed() {
		return helm.ErrorResult(res.Message()), nil
	}

	summary := strings.TrimSpace(res.Stdout)
	if summary == "" {
		summary = fmt.Sprintf("Rolled back release %s to %s", name, target)
	}

	return map[string]interface{}{
		"id":          name,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}, nil
}
