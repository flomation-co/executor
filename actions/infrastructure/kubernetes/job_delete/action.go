// Package infrastructure_kubernetes_job_delete deletes a Job (batch/v1).
//
// The choice that matters here is propagation. A Job owns the pods it spawned; how
// those pods are handled when the Job is deleted depends on the policy:
//
//   - Background (the default here, and the server's own default) deletes the Job
//     immediately and garbage-collects its pods afterwards.
//   - Foreground blocks until the pods are gone, then removes the Job.
//   - Orphan deletes the Job object but LEAVES ITS PODS RUNNING — a running
//     backup or migration keeps going, now unowned, and must be cleaned up by
//     hand. That is occasionally what you want, and a foot-gun otherwise, so it is
//     offered but never the default.
//
// Deletion is destructive and irreversible, so it is gated on confirm_destructive,
// which fails closed — see kubernetes.ConfirmDestructive.
package infrastructure_kubernetes_job_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Delete Job"
	Description  = "Permanently delete a Job. Deleting with Background (default) or Foreground removes its pods too; Orphan leaves them running. Requires explicit confirmation."
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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace the job lives in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Job", Placeholder: "The job to delete", Required: true},
	{Name: "propagation_policy", Type: core.ConnectionTypeString, Label: "Propagation Policy", Options: []core.ConnectionOption{
		{Name: "Background — delete the job now, garbage-collect its pods after", Value: "Background"},
		{Name: "Foreground — wait for the pods to be deleted first", Value: "Foreground"},
		{Name: "Orphan — delete the job but leave its pods running", Value: "Orphan"},
	}},
	{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Label: "Confirm Destructive Action", Placeholder: "This permanently changes cluster state. Tick to allow, or bind a variable such as ${var.approved}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Job Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
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

	propagation := kubernetes.OptionalString("propagation_policy", inputs)
	if propagation == "" {
		propagation = "Background"
	}
	switch propagation {
	case "Background", "Foreground", "Orphan":
	default:
		return nil, fmt.Errorf(`propagation_policy must be "Background", "Foreground" or "Orphan" (got %q)`, propagation)
	}

	if err := kubernetes.ConfirmDestructive(inputs, "delete the job "+name); err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	obj, err := kubernetes.Delete(ctx, auth, kubernetes.Jobs, namespace, name, kubernetes.DeleteOptions{PropagationPolicy: propagation})
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Deleted job %s from namespace %s (%s propagation)", name, namespace, propagation)
	if propagation == "Orphan" {
		summary = fmt.Sprintf("Deleted job %s from namespace %s with Orphan propagation — its pods are left running and must be cleaned up separately", name, namespace)
	}

	out := kubernetes.ObjectResult(obj, summary)
	if kubernetes.ObjectName(obj) == "" {
		out["id"] = name
	}
	return out, nil
}
