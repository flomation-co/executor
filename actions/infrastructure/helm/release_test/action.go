// Package infrastructure_helm_release_test runs a release's chart tests — the
// shell-out equivalent of `helm test <name>`.
//
// A chart's tests are Pods annotated helm.sh/hook: test; `helm test` runs them
// against the already-installed release and reports whether they passed. It reads
// cluster state and creates only short-lived test Pods, so it is not destructive
// and carries no confirm_destructive guard.
//
// The output is plain text, not JSON. With "Dump Test Pod Logs" set, helm appends
// each test pod's logs to that output — the useful detail when a test fails.
package infrastructure_helm_release_test

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
	Name         = "Helm Test Release"
	Description  = "Run a Helm release's chart tests and report whether they passed, optionally dumping the test pod logs."
	Website      = "https://www.flomation.co"
	Icon         = "helm+play"
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Release Name", Placeholder: "The release to test, e.g. my-nginx", Required: true},
	{Name: "timeout_seconds", Type: core.ConnectionTypeInteger, Label: "Timeout (seconds)", Placeholder: "How long to wait for the tests to finish (default 300)"},
	{Name: "logs", Type: core.ConnectionTypeBoolean, Label: "Dump Test Pod Logs", Placeholder: "Append each test pod's logs to the result"},
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

	logs := kubernetes.BoolInput("logs", inputs)
	timeoutSec := defaultTimeoutSeconds
	if n, ok := kubernetes.OptionalInt("timeout_seconds", inputs); ok && n > 0 {
		timeoutSec = n
	}

	version := kubernetes.OptionalString("helm_version", inputs)
	binaryPath := kubernetes.OptionalString("binary_path", inputs)

	args := []string{"test", name, "-n", namespace, "--timeout", strconv.Itoa(timeoutSec) + "s"}
	if logs {
		args = append(args, "--logs")
	}

	// Invoke applies its own 15-minute timeout, so a bare Background context is
	// correct here — a chart's tests are legitimately long-running.
	res, err := helm.Invoke(context.Background(), auth, version, binaryPath, namespace, args)
	if err != nil {
		return helm.ErrorResult(err.Error()), nil
	}
	if res.Failed() {
		return helm.ErrorResult(res.Message()), nil
	}

	summary := strings.TrimSpace(res.Stdout)
	if summary == "" {
		summary = fmt.Sprintf("Tests for release %s passed", name)
	}

	return map[string]interface{}{
		"id":          name,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}, nil
}
