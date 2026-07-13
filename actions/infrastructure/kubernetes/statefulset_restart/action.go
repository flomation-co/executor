// Package infrastructure_kubernetes_statefulset_restart performs a rolling
// restart of a StatefulSet, exactly as `kubectl rollout restart` does.
//
// There is no "restart" verb in the Kubernetes API. What kubectl actually does is
// stamp a timestamp annotation onto the StatefulSet's *pod template*; because the
// template changed, the StatefulSet controller recreates its pods one ordinal at
// a time — highest ordinal first — respecting the configured podManagementPolicy
// and update strategy. Each replacement pod reattaches the same
// PersistentVolumeClaim, so data survives the restart. Nothing ever reads the
// annotation's value.
//
// The patch is a strategic merge patch: a plain merge patch would replace the
// whole annotations map, dropping any other annotation on the template.
package infrastructure_kubernetes_statefulset_restart

import (
	"encoding/json"
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Restart StatefulSet"
	Description  = "Trigger a rolling restart of a StatefulSet — recreates its pods one ordinal at a time, reattaching the same volumes."
	Website      = "https://www.flomation.co"
	Icon         = "kubernetes+rotate-right"
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "StatefulSet", Placeholder: "The statefulset to restart", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "StatefulSet Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "StatefulSet"},
	{Name: "restarted_at", Type: core.ConnectionTypeString, Label: "Restarted At"},
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

	restartedAt := time.Now().UTC().Format(time.RFC3339)
	patch, err := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						kubernetes.RestartedAtAnnotation: restartedAt,
					},
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	obj, err := kubernetes.Patch(ctx, auth, kubernetes.StatefulSets, namespace, name, patch, kubernetes.PatchStrategicMerge)
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	out := kubernetes.ObjectResult(obj, fmt.Sprintf("Rolling restart of statefulset %s triggered at %s", name, restartedAt))
	out["restarted_at"] = restartedAt
	return out, nil
}
