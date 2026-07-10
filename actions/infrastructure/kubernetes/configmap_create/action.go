// Package infrastructure_kubernetes_configmap_create creates a ConfigMap from a
// flat map of key/value pairs.
//
// The Data input is read through kubernetes.StringMapInput, which insists on a
// flat string→string map: a ConfigMap's data values are strings, and a nested
// object or array pasted here would be rejected by the API server later and less
// clearly. Non-string scalars (a YAML-ish `port: 8080`) are stringified.
package infrastructure_kubernetes_configmap_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Create ConfigMap"
	Description  = "Create a ConfigMap in a namespace from a map of key/value pairs, with optional labels."
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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace to create the config map in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "ConfigMap Name", Placeholder: "app-config", Required: true},
	{Name: "data", Type: core.ConnectionTypeObject, Label: "Data", Placeholder: `A flat map of string values, e.g. {"LOG_LEVEL":"info","PORT":"8080"}`, Required: true},
	{Name: "labels", Type: core.ConnectionTypeObject, Label: "Labels", Placeholder: `Optional metadata labels, e.g. {"app":"web"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "ConfigMap Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "ConfigMap"},
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

	data, err := kubernetes.StringMapInput("data", inputs)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("data is required — supply at least one key/value pair")
	}

	labels, err := kubernetes.StringMapInput("labels", inputs)
	if err != nil {
		return nil, err
	}

	metadata := map[string]interface{}{"name": name}
	if len(labels) > 0 {
		metadata["labels"] = labels
	}
	obj := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   metadata,
		"data":       data,
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	created, err := kubernetes.Create(ctx, auth, kubernetes.ConfigMaps, namespace, obj)
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	out := kubernetes.ObjectResult(created, fmt.Sprintf("Created config map %s with %d key(s) in namespace %s", name, len(data), namespace))
	if kubernetes.ObjectName(created) == "" {
		out["id"] = name
	}
	return out, nil
}
