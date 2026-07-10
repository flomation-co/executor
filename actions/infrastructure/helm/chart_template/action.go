// Package infrastructure_helm_chart_template renders a chart to YAML without
// touching the cluster — the "plan" of Helm, and the natural thing to show a
// human before an install.
//
// `helm template` runs the chart's engine entirely client-side and prints the
// manifests it would apply. It reaches the API server only to read cluster
// capabilities (the .Capabilities builtin a chart may branch on), so it still
// takes the credential block; it never creates, updates, or deletes anything.
//
// Values are written to a file and passed with -f (see helm.WriteValuesFile)
// rather than --set, so a value containing a comma or a brace survives intact.
package infrastructure_helm_chart_template

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/helm"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Helm Template Chart"
	Description  = "Render a chart to Kubernetes YAML locally, without touching the cluster — preview exactly what an install would apply."
	Website      = "https://www.flomation.co"
	Icon         = "helm+file-lines"
	Date         = "10/07/2026"
	Type         = core.ActionTypeAction
)

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

	{Name: "name", Type: core.ConnectionTypeString, Label: "Release Name", Placeholder: "The release name to render the templates as, e.g. my-app", Required: true},
	{Name: "chart", Type: core.ConnectionTypeString, Label: "Chart", Placeholder: "A chart name (with Repository URL), an oci:// reference, or a local path", Required: true},
	{Name: "repo_url", Type: core.ConnectionTypeString, Label: "Repository URL", Placeholder: "https://charts.bitnami.com/bitnami — resolves the chart without a repo add"},
	{Name: "chart_version", Type: core.ConnectionTypeString, Label: "Chart Version", Placeholder: "Pin an exact chart version, e.g. 15.2.1 (blank for the latest)"},
	{Name: "values", Type: core.ConnectionTypeCode, Label: "Values (YAML)", Placeholder: "replicaCount: 2\nimage:\n  tag: \"1.27\""},
	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace the manifests target (blank for default)"},
	{Name: "include_crds", Type: core.ConnectionTypeBoolean, Label: "Include CRDs", Placeholder: "Also render the chart's CustomResourceDefinitions"},
}

var Outputs = [...]core.Connection{
	{Name: "manifest", Type: core.ConnectionTypeText, Label: "Rendered Manifest"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := kubernetes.GetAuth(inputs)
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

	version := kubernetes.OptionalString("helm_version", inputs)
	binaryPath := kubernetes.OptionalString("binary_path", inputs)
	repoURL := kubernetes.OptionalString("repo_url", inputs)
	chartVersion := kubernetes.OptionalString("chart_version", inputs)
	values := kubernetes.OptionalString("values", inputs)
	namespace := kubernetes.OptionalString("namespace", inputs)
	includeCRDs := kubernetes.BoolInput("include_crds", inputs)

	ctx, cancel := kubernetes.Context()
	defer cancel()

	res, err := helm.InvokeWithValues(ctx, auth, version, binaryPath, namespace, values, func(valuesArgs []string) []string {
		args := []string{"template", name}
		args = helm.AddChartSource(args, chart, repoURL, chartVersion)
		if namespace != "" {
			args = append(args, "-n", namespace)
		}
		args = append(args, valuesArgs...)
		if includeCRDs {
			args = append(args, "--include-crds")
		}
		return args
	})
	if err != nil {
		return helm.ErrorResult(err.Error()), nil
	}
	if res.Failed() {
		return helm.ErrorResult(res.Message()), nil
	}

	return map[string]interface{}{
		"manifest":    res.Stdout,
		"tool_result": fmt.Sprintf("Rendered chart %s as release %q — %d bytes of manifest", chart, name, len(res.Stdout)),
		"success":     true,
		"error":       "",
	}, nil
}
