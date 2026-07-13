// Package infrastructure_kubernetes_serviceaccount_create creates a
// ServiceAccount in a namespace.
//
// Since Kubernetes 1.24 (the LegacyServiceAccountTokenNoAutoGeneration change),
// creating a ServiceAccount no longer mints a long-lived token Secret alongside
// it — the created object's `secrets` field comes back empty, and that is
// correct, not a failure. To obtain a credential for the account, ask the API
// server for a short-lived, audience-scoped token via the TokenRequest API
// (`kubectl create token <name> -n <namespace>`), or, only if you truly need a
// non-expiring token, create a Secret of type kubernetes.io/service-account-token
// annotated with kubernetes.io/service-account.name.
package infrastructure_kubernetes_serviceaccount_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Create Service Account"
	Description  = "Create a ServiceAccount in a namespace, with optional labels and annotations. Note: since Kubernetes 1.24 this no longer auto-creates a token Secret — use kubectl create token for a short-lived one."
	Website      = "https://www.flomation.co"
	Icon         = "kubernetes+plus"
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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace to create the ServiceAccount in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Service Account", Placeholder: "The name for the new ServiceAccount (DNS-safe, e.g. deploy-bot)", Required: true},
	{Name: "labels", Type: core.ConnectionTypeObject, Label: "Labels", Placeholder: `{"app":"deploy-bot","team":"platform"}`},
	{Name: "annotations", Type: core.ConnectionTypeObject, Label: "Annotations", Placeholder: `{"eks.amazonaws.com/role-arn":"arn:aws:iam::…"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Service Account Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "ServiceAccount"},
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

	labels, err := kubernetes.StringMapInput("labels", inputs)
	if err != nil {
		return nil, err
	}
	annotations, err := kubernetes.StringMapInput("annotations", inputs)
	if err != nil {
		return nil, err
	}

	metadata := map[string]interface{}{"name": name}
	if len(labels) > 0 {
		metadata["labels"] = labels
	}
	if len(annotations) > 0 {
		metadata["annotations"] = annotations
	}
	body := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ServiceAccount",
		"metadata":   metadata,
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	obj, err := kubernetes.Create(ctx, auth, kubernetes.ServiceAccounts, namespace, body)
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	return kubernetes.ObjectResult(obj, fmt.Sprintf("Created service account %s in namespace %s", name, namespace)), nil
}
