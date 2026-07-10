// Package infrastructure_kubernetes_deployment_scale sets a Deployment's replica
// count, exactly as `kubectl scale deployment` does.
//
// It patches the /scale subresource rather than the Deployment itself, so it
// touches nothing but spec.replicas — an operator's other pending edits to the
// Deployment are left alone. The patch MUST be a plain merge patch (RFC 7386):
// the autoscaling Scale type carries no patchStrategy metadata, and the API
// server rejects a strategic-merge patch against it with a 415.
package infrastructure_kubernetes_deployment_scale

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Scale Deployment"
	Description  = "Set the number of replicas a Deployment runs — scale up for load, or down to zero to pause it."
	Website      = "https://www.flomation.co"
	Icon         = "kubernetes+arrows-up-down"
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Deployment", Placeholder: "The deployment to scale", Required: true},
	{Name: "replicas", Type: core.ConnectionTypeInteger, Label: "Replicas", Placeholder: "The desired number of pods (0 pauses the deployment)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Deployment Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Scale"},
	{Name: "replicas", Type: core.ConnectionTypeInteger, Label: "Replicas"},
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

	replicas, ok := kubernetes.OptionalInt("replicas", inputs)
	if !ok {
		return nil, fmt.Errorf("replicas is required")
	}
	// A negative replica count is a config error, not something the API server
	// should be asked to reject — refuse it up front.
	if replicas < 0 {
		return nil, fmt.Errorf("replicas must be zero or greater (got %d)", replicas)
	}

	// Plain merge patch (PatchMerge), NOT strategic — the Scale type carries no
	// patchStrategy metadata, so a strategic-merge patch is a 415.
	patch, err := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": replicas,
		},
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	obj, err := kubernetes.PatchSub(ctx, auth, kubernetes.Deployments, namespace, name, "scale", patch, kubernetes.PatchMerge)
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Scaled deployment %s to %d replicas", name, replicas)
	if replicas == 0 {
		summary = fmt.Sprintf("Scaled deployment %s to 0 replicas — the deployment now runs no pods", name)
	}

	out := kubernetes.ObjectResult(obj, summary)
	out["replicas"] = replicas
	return out, nil
}
