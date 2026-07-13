// Package infrastructure_kubernetes_serviceaccount_delete deletes a
// ServiceAccount.
//
// Deleting a ServiceAccount immediately invalidates every credential issued for
// it: any workload still mounting its token, and any client authenticating as it,
// loses access the moment the object is gone. That can break running pods, so the
// action is gated on confirm_destructive, which fails closed — see
// kubernetes.ConfirmDestructive. The guard is a boolean, so a flow can bind it to
// a variable and decide at run time (e.g. ${var.approved}) rather than at design
// time.
package infrastructure_kubernetes_serviceaccount_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Delete Service Account"
	Description  = "Permanently delete a ServiceAccount, invalidating every token issued for it. Requires explicit confirmation."
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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace the ServiceAccount lives in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Service Account", Placeholder: "The ServiceAccount to delete — every token issued for it stops working", Required: true},
	{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Label: "Confirm Destructive Action", Placeholder: "This permanently changes cluster state. Tick to allow, or bind a variable such as ${var.approved}", Required: true},
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
	if err := kubernetes.ConfirmDestructive(inputs, "delete the service account "+name); err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	obj, err := kubernetes.Delete(ctx, auth, kubernetes.ServiceAccounts, namespace, name, kubernetes.DeleteOptions{})
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	out := kubernetes.ObjectResult(obj, fmt.Sprintf("Service account %s in namespace %s deleted", name, namespace))
	if kubernetes.ObjectName(obj) == "" {
		out["id"] = name
	}
	return out, nil
}
