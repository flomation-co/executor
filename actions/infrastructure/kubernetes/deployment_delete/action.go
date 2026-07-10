// Package infrastructure_kubernetes_deployment_delete deletes a Deployment.
//
// This is destructive — the Deployment and the pods it manages go — so it is
// gated on confirm_destructive, which fails closed (see
// kubernetes.ConfirmDestructive). The guard is a boolean, so a flow can bind it
// to a variable and decide at run time (e.g. ${var.approved}) rather than by a
// checkbox ticked at design time.
//
// propagation_policy chooses what happens to the managed ReplicaSets and pods:
// Background (the default) removes the Deployment immediately and garbage-
// collects its dependents afterwards; Foreground blocks until they are gone;
// Orphan leaves the ReplicaSets and pods running, detached.
package infrastructure_kubernetes_deployment_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Delete Deployment"
	Description  = "Permanently delete a Deployment and the pods it manages. Requires explicit confirmation."
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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace the deployment lives in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Deployment", Placeholder: "The deployment to delete", Required: true},
	{Name: "propagation_policy", Type: core.ConnectionTypeString, Label: "Propagation Policy", Placeholder: "How the managed pods are handled (default Background)", Options: []core.ConnectionOption{
		{Name: "Background — delete now, garbage-collect pods after", Value: "Background"},
		{Name: "Foreground — block until pods are gone", Value: "Foreground"},
		{Name: "Orphan — leave the pods running, detached", Value: "Orphan"},
	}},
	{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Label: "Confirm Destructive Action", Placeholder: "This permanently changes cluster state. Tick to allow, or bind a variable such as ${var.approved}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Deployment Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Deployment"},
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

	if err := kubernetes.ConfirmDestructive(inputs, "delete the deployment "+name); err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	policy := kubernetes.OptionalString("propagation_policy", inputs)
	if policy == "" {
		policy = "Background"
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	obj, err := kubernetes.Delete(ctx, auth, kubernetes.Deployments, namespace, name, kubernetes.DeleteOptions{PropagationPolicy: policy})
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	out := kubernetes.ObjectResult(obj, fmt.Sprintf("Deleted deployment %s in namespace %s (%s propagation)", name, namespace, policy))
	// A delete response echoes the object as it was, or a bare Status; fall back
	// to the requested name so the id port always carries the target.
	if kubernetes.ObjectName(obj) == "" {
		out["id"] = name
	}
	return out, nil
}
