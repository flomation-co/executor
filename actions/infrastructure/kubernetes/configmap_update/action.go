// Package infrastructure_kubernetes_configmap_update changes the data of an
// existing ConfigMap, either merging the supplied keys into what is already there
// or replacing the whole data map with exactly the supplied keys.
//
// The Replace flag is the entire point of this action, and the two modes use
// different patch mechanisms because a ConfigMap's `data` is a plain
// string→string map:
//
//   - MERGE (default) sends a strategic-merge patch {"data":{…}}. A map has no
//     patchStrategy, so the server merges it key by key: the supplied keys
//     overwrite their existing values, and every key NOT mentioned survives
//     untouched. This is the safe, additive update.
//
//   - REPLACE swaps the whole data map. An RFC 7386 merge patch cannot express
//     this in one shot — it, too, merges an object member by key, so keys absent
//     from the patch would silently survive. The correct primitive is a JSON
//     Patch (RFC 6902) `add` on /data: `add` sets an object member outright,
//     creating it if absent and REPLACING it whole if present, so keys not in the
//     supplied map are dropped. That is what makes "Replace all data" actually
//     remove stale keys rather than just overwrite the ones you named.
package infrastructure_kubernetes_configmap_update

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Update ConfigMap"
	Description  = "Update a ConfigMap's data — merge the supplied keys into the existing data, or replace the whole data map."
	Website      = "https://www.flomation.co"
	Icon         = "kubernetes+pencil"
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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace the config map lives in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "ConfigMap", Placeholder: "The config map to update", Required: true},
	{Name: "data", Type: core.ConnectionTypeObject, Label: "Data", Placeholder: `A flat map of string values, e.g. {"LOG_LEVEL":"debug"}`, Required: true},
	{Name: "replace", Type: core.ConnectionTypeBoolean, Label: "Replace All Data", Placeholder: "Replace all data instead of merging the supplied keys — keys absent from the map above are dropped"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "ConfigMap Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "ConfigMap"},
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

	data, err := kubernetes.StringMapInput("data", inputs)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("data is required — supply at least one key/value pair")
	}

	replace := kubernetes.BoolInput("replace", inputs)

	var patch []byte
	var patchType string
	if replace {
		// JSON Patch `add` on /data sets the whole map outright — created if
		// absent, replaced whole if present — so keys not supplied are dropped.
		patch, err = json.Marshal([]map[string]interface{}{
			{"op": "add", "path": "/data", "value": data},
		})
		patchType = kubernetes.PatchJSON
	} else {
		// Strategic-merge patch: supplied keys overwrite, untouched keys survive.
		patch, err = json.Marshal(map[string]interface{}{"data": data})
		patchType = kubernetes.PatchStrategicMerge
	}
	if err != nil {
		return nil, err
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	obj, err := kubernetes.Patch(ctx, auth, kubernetes.ConfigMaps, namespace, name, patch, patchType)
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	verb := "Merged"
	tail := fmt.Sprintf("%d key(s) into config map %s", len(data), name)
	if replace {
		verb = "Replaced"
		tail = fmt.Sprintf("the data of config map %s with %d key(s)", name, len(data))
	}
	return kubernetes.ObjectResult(obj, fmt.Sprintf("%s %s in namespace %s", verb, tail, namespace)), nil
}
