// Package infrastructure_kubernetes_statefulset_delete deletes a StatefulSet.
//
// This is destructive and gated on confirm_destructive, which fails closed — see
// kubernetes.ConfirmDestructive. The guard is a boolean so a flow can bind it to
// a variable and let the decision be made at run time (e.g. ${var.approved} set
// by an upstream Human-in-the-Loop node) rather than a checkbox ticked at design
// time.
//
// One thing operators regularly get wrong: deleting a StatefulSet does NOT delete
// the PersistentVolumeClaims it created for its pods. Those PVCs (and the volumes
// behind them) survive, so the data is retained and a StatefulSet recreated with
// the same name reattaches them. To reclaim the storage, delete the PVCs
// separately. propagation_policy controls only how the *pods* are torn down.
package infrastructure_kubernetes_statefulset_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Delete StatefulSet"
	Description  = "Permanently delete a StatefulSet. Its PersistentVolumeClaims are NOT removed — delete them separately to reclaim storage. Requires explicit confirmation."
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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace the statefulset lives in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "StatefulSet", Placeholder: "The statefulset to delete — its PersistentVolumeClaims are left behind", Required: true},
	{Name: "propagation_policy", Type: core.ConnectionTypeString, Label: "Deletion Propagation", Placeholder: "How the managed pods are torn down (default Background)", Options: []core.ConnectionOption{
		{Name: "Background — delete now, reap pods afterwards", Value: "Background"},
		{Name: "Foreground — block until the pods are gone", Value: "Foreground"},
		{Name: "Orphan — delete the StatefulSet but leave its pods running", Value: "Orphan"},
	}},
	{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Label: "Confirm Destructive Action", Placeholder: "This permanently changes cluster state. Tick to allow, or bind a variable such as ${var.approved}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "StatefulSet Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "StatefulSet"},
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
	if err := kubernetes.ConfirmDestructive(inputs, "delete the statefulset "+name); err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	opts := kubernetes.DeleteOptions{PropagationPolicy: kubernetes.OptionalString("propagation_policy", inputs)}
	obj, err := kubernetes.Delete(ctx, auth, kubernetes.StatefulSets, namespace, name, opts)
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	out := kubernetes.ObjectResult(obj, fmt.Sprintf("StatefulSet %s deleted — its PersistentVolumeClaims are NOT removed; delete them separately to reclaim storage", name))
	if kubernetes.ObjectName(obj) == "" {
		out["id"] = name
	}
	return out, nil
}
