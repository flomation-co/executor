// Package infrastructure_kubernetes_hpa_delete deletes a
// HorizontalPodAutoscaler.
//
// This is destructive but narrow: deleting an HPA does NOT scale the workload
// down or up. It simply stops autoscaling — the target (Deployment, StatefulSet,
// …) is left running at whatever replica count it happened to be at when the HPA
// went away, and stays there until something else changes it. It is gated on
// confirm_destructive, which fails closed — see kubernetes.ConfirmDestructive.
//
// The HPA is addressed under autoscaling/v2, the GA version from Kubernetes 1.23
// onward (v2beta2 was removed in 1.26), so this needs a 1.23+ cluster.
package infrastructure_kubernetes_hpa_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Delete Horizontal Pod Autoscaler"
	Description  = "Delete a HorizontalPodAutoscaler. Autoscaling stops; the workload is left at its current replica count. Requires explicit confirmation."
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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace the autoscaler lives in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Autoscaler", Placeholder: "The HorizontalPodAutoscaler to delete", Required: true},
	{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Label: "Confirm Destructive Action", Placeholder: "This permanently changes cluster state. Tick to allow, or bind a variable such as ${var.approved}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Autoscaler Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Autoscaler"},
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
	if err := kubernetes.ConfirmDestructive(inputs, "delete the autoscaler "+name+" in namespace "+namespace); err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	obj, err := kubernetes.Delete(ctx, auth, kubernetes.HPAs, namespace, name, kubernetes.DeleteOptions{})
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	out := kubernetes.ObjectResult(obj, fmt.Sprintf("Deleted autoscaler %s in namespace %s — the workload stays at its current replica count", name, namespace))
	if kubernetes.ObjectName(obj) == "" {
		out["id"] = name
	}
	return out, nil
}
