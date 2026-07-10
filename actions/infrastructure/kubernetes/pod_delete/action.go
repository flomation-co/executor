// Package infrastructure_kubernetes_pod_delete deletes a single Pod.
//
// Deleting a Pod that is managed by a controller — a Deployment, StatefulSet,
// DaemonSet, ReplicaSet or Job — is not permanent: the controller notices the
// missing replica and schedules a fresh Pod to replace it. This is the normal,
// least-disruptive way to restart one wedged Pod without touching the rest of
// the workload. Only a bare, unmanaged Pod is gone for good.
//
// Because it changes cluster state either way, it is gated on
// confirm_destructive, which fails closed — see kubernetes.ConfirmDestructive.
//
// grace_period_seconds is optional and passed through only when set. Kubernetes
// distinguishes 0 (delete immediately, skip the graceful shutdown window) from
// "unset" (honour the Pod's own terminationGracePeriodSeconds), which is exactly
// why DeleteOptions.GracePeriodSeconds is a *int64 rather than a plain int64.
package infrastructure_kubernetes_pod_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Delete Pod"
	Description  = "Delete a Pod. If it is managed by a controller the controller recreates it — the usual way to restart one stuck Pod. Requires explicit confirmation."
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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace the pod lives in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Pod", Placeholder: "The pod to delete", Required: true},
	{Name: "grace_period_seconds", Type: core.ConnectionTypeInteger, Label: "Grace Period (seconds)", Placeholder: "Leave blank for the pod's own default; 0 deletes immediately with no graceful shutdown"},
	{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Label: "Confirm Destructive Action", Placeholder: "This permanently changes cluster state. Tick to allow, or bind a variable such as ${var.approved}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Pod Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Pod"},
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
	if err := kubernetes.ConfirmDestructive(inputs, "delete the pod "+name); err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	// Pass GracePeriodSeconds only when the operator supplied one, so an unset
	// field leaves the pod's own terminationGracePeriodSeconds in force. The
	// pointer is what lets 0 (delete immediately) differ from unset.
	var opts kubernetes.DeleteOptions
	if n, ok := kubernetes.OptionalInt("grace_period_seconds", inputs); ok {
		grace := int64(n)
		opts.GracePeriodSeconds = &grace
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	obj, err := kubernetes.Delete(ctx, auth, kubernetes.Pods, namespace, name, opts)
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	out := kubernetes.ObjectResult(obj, fmt.Sprintf("Pod %s in namespace %s is being deleted", name, namespace))
	if kubernetes.ObjectName(obj) == "" {
		out["id"] = name
	}
	return out, nil
}
