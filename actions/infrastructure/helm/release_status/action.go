// Package infrastructure_helm_release_status reports the status of a Helm
// release revision — its lifecycle state, chart, app version and timestamps —
// the shape `helm status` prints.
//
// It needs no helm binary: the revision is decoded from its Kubernetes storage
// Secret (see the helm package's release.go). A blank or zero revision reports
// the current one. The flattened fields (status, chart, app_version, …) are
// pulled up from the release so a flow can branch on them without walking the
// full object on the result port.
package infrastructure_helm_release_status

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/helm"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Helm Release Status"
	Description  = "Report a Helm release's status — its lifecycle state, chart, app version and when it was last deployed."
	Website      = "https://www.flomation.co"
	Icon         = "helm+check"
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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace the release was installed into", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Release", Placeholder: "The Helm release to report on", Required: true},
	{Name: "revision", Type: core.ConnectionTypeInteger, Label: "Revision", Placeholder: "A specific revision number, or blank for the current one"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Release Name"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "revision", Type: core.ConnectionTypeInteger, Label: "Revision"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description"},
	{Name: "last_deployed", Type: core.ConnectionTypeString, Label: "Last Deployed"},
	{Name: "chart", Type: core.ConnectionTypeString, Label: "Chart"},
	{Name: "app_version", Type: core.ConnectionTypeString, Label: "App Version"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Release"},
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
	revision, _ := kubernetes.OptionalInt("revision", inputs)

	ctx, cancel := kubernetes.Context()
	defer cancel()

	rel, err := helm.Revision(ctx, auth, namespace, name, revision)
	if err != nil {
		return helm.ErrorResult(err.Error()), nil
	}

	// description and last_deployed live in the release's Info map, for which
	// there is no dedicated getter.
	info := func(k string) string {
		if rel.Info == nil {
			return ""
		}
		v, _ := rel.Info[k].(string)
		return v
	}

	status := rel.Status()
	summary := fmt.Sprintf("Release %s revision %d is %s", name, rel.Version, status)

	return map[string]interface{}{
		"id":            name,
		"status":        status,
		"revision":      rel.Version,
		"description":   info("description"),
		"last_deployed": info("last_deployed"),
		"chart":         rel.ChartName(),
		"app_version":   rel.AppVersion(),
		"result":        rel,
		"tool_result":   summary,
		"success":       true,
		"error":         "",
	}, nil
}
