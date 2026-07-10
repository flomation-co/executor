// Package infrastructure_helm_chart_lint runs `helm lint` over a chart,
// examining it for well-formedness and best-practice violations.
//
// lint is purely local — it never contacts the cluster — but the action still
// carries the credential block so the same connection an install would use is
// configured here, and so the two nodes are interchangeable in a flow.
//
// Unlike install, upgrade, template and show, `helm lint` takes a path to an
// unpacked chart and has no --repo flag: it is the one subcommand that cannot
// resolve a chart from a repository itself. So a remote chart is pulled and
// unpacked into the invocation's temp directory first, and lint is pointed at
// what came out. Both commands share one helm.Session so they see the same home.
//
// helm lint exits non-zero when it finds ERROR-level problems (and, with
// --strict, on WARNINGs too). That is reported as a SOFT failure carrying the
// findings on the error port rather than aborting the flow: a lint failure is a
// result the flow may want to branch on, not a broken action.
package infrastructure_helm_chart_lint

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/helm"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Helm Lint Chart"
	Description  = "Check a chart for errors and best-practice violations with helm lint, without touching the cluster."
	Website      = "https://www.flomation.co"
	Icon         = "helm+magnifying-glass"
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
	{Name: "values", Type: core.ConnectionTypeCode, Label: "Values (YAML)", Placeholder: "replicaCount: 2\nimage:\n  tag: \"1.27\""},
	{Name: "strict", Type: core.ConnectionTypeBoolean, Label: "Strict", Placeholder: "Treat warnings as failures too"},
}

var Outputs = [...]core.Connection{
	{Name: "output", Type: core.ConnectionTypeText, Label: "Lint Output"},
	{Name: "passed", Type: core.ConnectionTypeBoolean, Label: "Passed"},
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
	values := kubernetes.OptionalString("values", inputs)
	strict := kubernetes.BoolInput("strict", inputs)

	ctx, cancel := kubernetes.Context()
	defer cancel()

	// auth is threaded through only so the session can synthesise its kubeconfig.
	// Neither `helm pull` nor `helm lint` contacts the cluster; the connection is
	// carried so this node and release_install take the same one, and are drop-in
	// interchangeable in a flow.
	res, err := helm.WithSession(ctx, auth, version, binaryPath, "", values, func(s *helm.Session) (*helm.RunResult, error) {
		target := chart

		// A chart already on the runner's disk is linted where it lies. Anything
		// else — a repo chart, an oci:// reference, a .tgz URL — must be fetched
		// and unpacked, because lint reads a directory.
		if !isLocalPath(chart) {
			untarDir := s.Home + "/chart"
			chartArgs, err := s.ResolveChart(source)
			if err != nil {
				return nil, err
			}
			pull := append([]string{"pull"}, chartArgs...)
			pull = append(pull, "--untar", "--untardir", untarDir)

			pulled, err := s.Run(pull...)
			if err != nil {
				return pulled, err
			}
			if pulled.Failed() {
				// Distinguish "could not fetch the chart" from "the chart failed
				// lint" — the caller reports a non-zero exit as a lint finding.
				return nil, fmt.Errorf("could not fetch chart %s: %s", chart, pulled.Message())
			}
			if target, err = helm.SoleChartDir(untarDir); err != nil {
				return nil, err
			}
		}

		args := append([]string{"lint", target}, s.ValuesArgs...)
		if strict {
			args = append(args, "--strict")
		}
		return s.Run(args...)
	})
	if err != nil {
		return helm.ErrorResult(err.Error()), nil
	}

	// A non-zero exit means lint found problems. The detail an operator needs is
	// on stdout (the per-line findings); helm's stderr carries only the summary
	// line, which is the better one-line error message.
	if res.Failed() {
		return map[string]interface{}{
			"output":      res.Stdout,
			"passed":      false,
			"tool_result": fmt.Sprintf("Chart %s failed lint", chart),
			"success":     false,
			"error":       res.Message(),
		}, nil
	}

	return map[string]interface{}{
		"output":      res.Stdout,
		"passed":      true,
		"tool_result": fmt.Sprintf("Chart %s passed lint", chart),
		"success":     true,
		"error":       "",
	}, nil
}

// isLocalPath reports whether the chart reference names a directory or archive on
// the runner's own filesystem, rather than something helm must fetch.
func isLocalPath(chart string) bool {
	if strings.Contains(chart, "://") {
		return false // oci://, https://, ...
	}
	return strings.HasPrefix(chart, "/") || strings.HasPrefix(chart, "./") || strings.HasPrefix(chart, "../")
}
