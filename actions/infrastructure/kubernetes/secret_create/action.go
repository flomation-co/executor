// Package infrastructure_kubernetes_secret_create creates a Secret.
//
// The values go into the object's `stringData` field, NOT `data`. Kubernetes
// base64-encodes stringData server-side, so the operator pastes plain values and
// there is no encoding step for a flow to get wrong (a `data` map would demand
// pre-encoded values, and a single un-encoded entry is accepted by the API but
// then decodes to garbage at mount time). The created object the API returns
// carries the encoded values back in its `data` map on the result port.
//
// The input is deliberately named `string_data` and the Secret type is carried
// in `secret_type` — never a field literally named `type`, which would collide
// with the action framework's own Type const if it were an input name and reads
// poorly as a resource field besides.
package infrastructure_kubernetes_secret_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Create Secret"
	Description  = "Create a Secret from plain key/value pairs — Kubernetes base64-encodes the values for you."
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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace to create the secret in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Secret", Placeholder: "The name of the secret to create", Required: true},
	{Name: "string_data", Type: core.ConnectionTypeObject, Label: "Data", Placeholder: `Plain key/value pairs, e.g. {"username":"admin","password":"s3cret"} — encoded for you`, Required: true},
	{Name: "secret_type", Type: core.ConnectionTypeString, Label: "Secret Type", Placeholder: "Opaque (default), kubernetes.io/tls, kubernetes.io/dockerconfigjson, …"},
	{Name: "labels", Type: core.ConnectionTypeObject, Label: "Labels", Placeholder: `{"app":"web","managed-by":"flomation"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Secret Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Secret"},
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

	stringData, err := kubernetes.StringMapInput("string_data", inputs)
	if err != nil {
		return nil, err
	}
	if len(stringData) == 0 {
		return nil, fmt.Errorf("string_data is required — provide at least one key/value pair")
	}

	labels, err := kubernetes.StringMapInput("labels", inputs)
	if err != nil {
		return nil, err
	}

	secretType := kubernetes.OptionalString("secret_type", inputs)
	if secretType == "" {
		secretType = "Opaque"
	}

	metadata := map[string]interface{}{"name": name}
	if len(labels) > 0 {
		metadata["labels"] = labels
	}

	body := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata":   metadata,
		"type":       secretType,
		"stringData": stringData,
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	obj, err := kubernetes.Create(ctx, auth, kubernetes.Secrets, namespace, body)
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	// The summary reports counts and the type only — never a value.
	out := kubernetes.ObjectResult(obj, fmt.Sprintf("Created secret %s in namespace %s (%d key(s), type %s)", name, namespace, len(stringData), secretType))
	if kubernetes.ObjectName(obj) == "" {
		out["id"] = name
	}
	return out, nil
}
