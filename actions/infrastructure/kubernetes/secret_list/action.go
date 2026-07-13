// Package infrastructure_kubernetes_secret_list lists Secrets, either in one
// namespace or across the whole cluster.
//
// WARNING: a Secret list response is not metadata-only. The Kubernetes API
// returns each Secret's `data` map — every value base64-encoded — inline in the
// list, exactly as a single Get would. So listing Secrets across every namespace
// pulls the (lightly-obscured) contents of every secret in the cluster into the
// flow's output. This action passes those objects straight through, as the API
// gives them, and does not decode or summarise the values; but callers should
// treat a broad list as sensitive and scope it with a namespace or label
// selector wherever they can.
package infrastructure_kubernetes_secret_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes List Secrets"
	Description  = "List the Secrets in a namespace, or across every namespace, optionally filtered by label or field. Note: each entry includes the secret's base64 data."
	Website      = "https://www.flomation.co"
	Icon         = "kubernetes+list"
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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "Leave blank to list across every namespace — pulls every secret in the cluster"},
	{Name: "label_selector", Type: core.ConnectionTypeString, Label: "Label Selector", Placeholder: "app=web,tier!=cache"},
	{Name: "field_selector", Type: core.ConnectionTypeString, Label: "Field Selector", Placeholder: "type=kubernetes.io/tls"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Objects per page (default 100, max 500)"},
	{Name: "continue_token", Type: core.ConnectionTypeString, Label: "Continue Token", Placeholder: "Resume from a previous run's continue token"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow continue tokens until every Secret is fetched"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Secrets"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "continue_token", Type: core.ConnectionTypeString, Label: "Continue Token"},
	{Name: "has_more", Type: core.ConnectionTypeBoolean, Label: "More Available"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := kubernetes.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	namespace := kubernetes.OptionalString("namespace", inputs)
	opts := kubernetes.ListOptionsFrom(inputs)

	items, continueToken, pages, err := kubernetes.List(ctx, auth, kubernetes.Secrets, namespace, opts)
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	scope := "every namespace"
	if namespace != "" {
		scope = "namespace " + namespace
	}

	summary := fmt.Sprintf("Found %d secret(s) in %s", len(items), scope)
	if opts.ReturnAll && pages >= kubernetes.MaxAllPages && continueToken != "" {
		summary = fmt.Sprintf("Fetched %d secret(s) across %d page(s) in %s; stopped at the %d-page safety cap — "+
			"narrow the filters, or pass the continue token back in to resume", len(items), pages, scope, kubernetes.MaxAllPages)
	} else if continueToken != "" {
		summary = fmt.Sprintf("Found %d secret(s) in %s; more remain — pass the continue token back in to fetch the next page", len(items), scope)
	}

	return kubernetes.ListResult(items, continueToken, summary), nil
}
