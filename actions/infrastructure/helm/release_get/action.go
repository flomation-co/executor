// Package infrastructure_helm_release_get reads one Helm release revision and
// returns a chosen slice of it — the whole release, or just its values, rendered
// manifest, notes or hooks.
//
// It needs no helm binary: the revision is decoded from its Kubernetes storage
// Secret (see the helm package's release.go). A blank or zero revision selects
// the current one.
//
// The result output holds whichever slice `content` selected, so its shape
// varies — an object for the whole release or its values, a string for the
// manifest or notes, an array for the hooks. Downstream nodes should branch on
// the content they requested.
package infrastructure_helm_release_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/helm"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Helm Get Release"
	Description  = "Read a Helm release revision — the whole release, or just its values, rendered manifest, notes or hooks."
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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace the release was installed into", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Release", Placeholder: "The Helm release to read", Required: true},
	{Name: "revision", Type: core.ConnectionTypeInteger, Label: "Revision", Placeholder: "A specific revision number, or blank for the current one"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "Content", Placeholder: "Which part of the release to return", Options: []core.ConnectionOption{
		{Name: "All", Value: "all"},
		{Name: "Values", Value: "values"},
		{Name: "Manifest", Value: "manifest"},
		{Name: "Notes", Value: "notes"},
		{Name: "Hooks", Value: "hooks"},
	}},
}

// The shape of `result` follows the Content selector: an object for the whole
// release and for its values, an array for its hooks, a string for its manifest
// and notes. ConnectionTypeObject is this codebase's "any structured value" — the
// list actions declare their `results` arrays the same way — so it is the honest
// declaration for a port whose type varies.
//
// Because a downstream node wiring a text field wants a real string rather than a
// JSON-encoded one, the two string-shaped contents are also published on `text`,
// which is empty for the others.
var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Release Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "text", Type: core.ConnectionTypeText, Label: "Manifest / Notes"},
	{Name: "revision", Type: core.ConnectionTypeInteger, Label: "Revision"},
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

	// content selects the slice to return; a blank or unrecognised value yields
	// the whole release, which is the safe default. text mirrors the two contents
	// that are plain strings, so a downstream text field can bind to a real string.
	var result interface{}
	var text, what string
	switch kubernetes.OptionalString("content", inputs) {
	case "values":
		result, what = rel.Config, "values"
	case "manifest":
		result, text, what = rel.Manifest, rel.Manifest, "manifest"
	case "notes":
		result, text, what = rel.Notes(), rel.Notes(), "notes"
	case "hooks":
		result, what = rel.Hooks, "hooks"
	default:
		result, what = rel, "release"
	}

	summary := fmt.Sprintf("Read the %s of release %s revision %d", what, name, rel.Version)
	out := helm.ReleaseResult(name, result, summary)
	out["text"] = text
	out["revision"] = rel.Version
	return out, nil
}
