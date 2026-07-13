// Package infrastructure_kubernetes_statefulset_scale sets the replica count of a
// StatefulSet, exactly as `kubectl scale statefulset` does.
//
// It patches the /scale subresource rather than the StatefulSet itself. That
// subresource is a Scale object, which carries no patchStrategy metadata, so the
// merge must be a plain RFC 7386 merge patch (PatchMerge) — a strategic-merge
// patch against /scale is rejected. Targeting /scale (not the spec.replicas of
// the whole object) also means the patch touches nothing but the replica count,
// so it never races a concurrent change to the rest of the spec.
//
// Scaling a StatefulSet down does NOT delete the PersistentVolumeClaims of the
// retired pods; scaling back up re-attaches the same volumes to the same ordinal
// pods. Data is preserved across a scale-to-zero and back.
package infrastructure_kubernetes_statefulset_scale

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Scale StatefulSet"
	Description  = "Set the number of replicas for a StatefulSet — scale it up, down, or to zero."
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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace the statefulset lives in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "StatefulSet", Placeholder: "The statefulset to scale", Required: true},
	{Name: "replicas", Type: core.ConnectionTypeInteger, Label: "Replicas", Placeholder: "The desired number of pods — 0 stops it without deleting its data", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "StatefulSet Name"},
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

	// 0 is a valid replica count (scale to zero), so the presence flag — not a
	// zero-value test — is what distinguishes "unset" from "scale to zero".
	replicas, ok := kubernetes.OptionalInt("replicas", inputs)
	if !ok {
		return nil, fmt.Errorf("replicas is required")
	}
	if replicas < 0 {
		return nil, fmt.Errorf("replicas must be zero or greater (got %d)", replicas)
	}

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

	obj, err := kubernetes.PatchSub(ctx, auth, kubernetes.StatefulSets, namespace, name, "scale", patch, kubernetes.PatchMerge)
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	out := kubernetes.ObjectResult(obj, fmt.Sprintf("Scaled statefulset %s to %d replica(s)", name, replicas))
	if kubernetes.ObjectName(obj) == "" {
		out["id"] = name
	}
	out["replicas"] = replicas
	return out, nil
}
