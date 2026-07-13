// Package infrastructure_kubernetes_daemonset_delete deletes a DaemonSet.
//
// Deleting a DaemonSet stops its pod running on every node it was scheduled to —
// for a cluster-wide agent (a log shipper, a CNI, a node exporter) that is a
// cluster-wide change, so the action is gated on confirm_destructive, which fails
// closed. See kubernetes.ConfirmDestructive.
//
// propagation_policy decides the fate of the pods the DaemonSet owns. Background
// (the server default) removes the DaemonSet immediately and garbage-collects its
// pods afterwards; Foreground blocks until the pods are gone; Orphan leaves the
// pods running, detached from any controller — useful when replacing the
// DaemonSet without a gap in coverage.
package infrastructure_kubernetes_daemonset_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Delete DaemonSet"
	Description  = "Permanently delete a DaemonSet, stopping its pod on every node. Requires explicit confirmation."
	Website      = "https://www.flomation.co"
	Icon         = "kubernetes+trash"
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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace the daemonset lives in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "DaemonSet", Placeholder: "The daemonset to delete", Required: true},
	{Name: "propagation_policy", Type: core.ConnectionTypeString, Label: "Propagation Policy", Placeholder: "How the owned pods are cleaned up (default Background)", Options: []core.ConnectionOption{
		{Name: "Background — delete now, reap pods after", Value: "Background"},
		{Name: "Foreground — block until pods are gone", Value: "Foreground"},
		{Name: "Orphan — leave the pods running", Value: "Orphan"},
	}},
	{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Label: "Confirm Destructive Action", Placeholder: "This permanently changes cluster state. Tick to allow, or bind a variable such as ${var.approved}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "DaemonSet Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "DaemonSet"},
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
	if err := kubernetes.ConfirmDestructive(inputs, "delete daemonset "+name+" in namespace "+namespace); err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	obj, err := kubernetes.Delete(ctx, auth, kubernetes.DaemonSets, namespace, name, kubernetes.DeleteOptions{
		PropagationPolicy: kubernetes.OptionalString("propagation_policy", inputs),
	})
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	out := kubernetes.ObjectResult(obj, fmt.Sprintf("Deleted daemonset %s in namespace %s", name, namespace))
	if kubernetes.ObjectName(obj) == "" {
		out["id"] = name
	}
	return out, nil
}
