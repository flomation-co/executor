// Package infrastructure_kubernetes_namespace_create creates a namespace.
//
// Namespace is cluster-scoped, so there is no namespace input and the Create
// call is passed "" for its namespace argument. Labels and annotations are
// optional flat string maps; each is omitted from the object body when empty so
// the server does not record an empty map.
package infrastructure_kubernetes_namespace_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Create Namespace"
	Description  = "Create a namespace, optionally with labels and annotations."
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

	{Name: "name", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The name of the namespace to create", Required: true},
	{Name: "labels", Type: core.ConnectionTypeObject, Label: "Labels", Placeholder: `{"team":"payments","env":"staging"}`},
	{Name: "annotations", Type: core.ConnectionTypeObject, Label: "Annotations", Placeholder: `{"owner":"platform@example.com"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Namespace Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Namespace"},
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
	obj := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata":   metadata,
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	// Namespaces are cluster-scoped, so the namespace argument to Create is "".
	created, err := kubernetes.Create(ctx, auth, kubernetes.Namespaces, "", obj)
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	out := kubernetes.ObjectResult(created, fmt.Sprintf("Created namespace %s", name))
	if kubernetes.ObjectName(created) == "" {
		out["id"] = name
	}
	return out, nil
}
