// Package infrastructure_helm_chart_show reads a chart's own documentation —
// its Chart.yaml metadata, its default values.yaml, its README, or all three —
// with `helm show`.
//
// This is a pure local read of the chart archive: it resolves and downloads the
// chart, then prints the requested section. It never contacts the cluster, but
// carries the credential block so it is interchangeable with the other Helm
// nodes in a flow.
//
// The section selector is named `info` rather than a second `chart` field:
// `chart` is the chart reference, `info` is which part of it to show.
package infrastructure_helm_chart_show

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/helm"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Helm Show Chart"
	Description  = "Read a chart's metadata, default values, README, or all of them, without installing it."
	Website      = "https://www.flomation.co"
	Icon         = "helm+eye"
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

	{Name: "chart", Type: core.ConnectionTypeString, Label: "Chart", Placeholder: "A chart name (with Repository URL), an oci:// reference, or a local path", Required: true},
	{Name: "repo_url", Type: core.ConnectionTypeString, Label: "Repository URL", Placeholder: "https://charts.bitnami.com/bitnami — resolves the chart without a repo add"},
	{Name: "chart_version", Type: core.ConnectionTypeString, Label: "Chart Version", Placeholder: "Pin an exact chart version, e.g. 15.2.1 (blank for the latest)"},
	{Name: "repo_username", Type: core.ConnectionTypeString, Label: "Repository Username", Placeholder: "For a private chart repository (Nexus, Artifactory, an OCI registry)"},
	{Name: "repo_password", Type: core.ConnectionTypeSecret, Label: "Repository Password", Placeholder: "Never passed on the command line — written to a 0600 file, or piped to registry login"},
	{Name: "repo_ca_cert", Type: core.ConnectionTypeText, Label: "Repository CA Certificate (PEM)", Placeholder: "-----BEGIN CERTIFICATE----- … for a repository behind an internal CA"},
	{Name: "repo_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure Repository TLS", Placeholder: "Skip certificate verification when fetching the chart — only for a self-signed internal repository"},
	{Name: "info", Type: core.ConnectionTypeString, Label: "Section", Options: []core.ConnectionOption{
		{Name: "Chart", Value: "chart"},
		{Name: "Values", Value: "values"},
		{Name: "Readme", Value: "readme"},
		{Name: "All", Value: "all"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "output", Type: core.ConnectionTypeText, Label: "Output"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := kubernetes.GetAuth(inputs)
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

	// The dropdown defaults to "chart"; a blank value (an unset optional) means
	// the same rather than an invalid `helm show ""`.
	info := kubernetes.OptionalString("info", inputs)
	if info == "" {
		info = "chart"
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	res, err := helm.WithSession(ctx, auth, version, binaryPath, "", "", func(s *helm.Session) (*helm.RunResult, error) {
		chartArgs, err := s.ResolveChart(source)
		if err != nil {
			return nil, err
		}
		return s.Run(append([]string{"show", info}, chartArgs...)...)
	})
	if err != nil {
		return helm.ErrorResult(err.Error()), nil
	}
	if res.Failed() {
		return helm.ErrorResult(res.Message()), nil
	}

	return map[string]interface{}{
		"output":      res.Stdout,
		"tool_result": fmt.Sprintf("Showed %s for chart %s", info, chart),
		"success":     true,
		"error":       "",
	}, nil
}
