// Package infrastructure_kubernetes_secret_get reads a single Secret.
//
// A Secret's values live in its `data` map, each value base64-encoded (that
// encoding is obfuscation for transport, not encryption). The raw object always
// carries that `data` map on the `result` port. When Decode is ticked, this
// action additionally base64-decodes each value into the `decoded_data` map so a
// downstream node can consume the plaintext directly.
//
// Decoding is best-effort per key: a value that is not valid base64 (a
// hand-edited or binary entry) is left OUT of decoded_data and its key is named
// in the summary, rather than failing the whole read. Key names are metadata, not
// secrets, so naming them is safe.
//
// A decoded secret value is NEVER placed into tool_result, into an error string,
// or into any log line — only onto the decoded_data data port the caller asked
// for. The summary reports counts and key names only.
package infrastructure_kubernetes_secret_get

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strings"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Get Secret"
	Description  = "Read a single Secret. Optionally base64-decode its values into a separate plaintext output."
	Website      = "https://www.flomation.co"
	Icon         = "kubernetes+eye"
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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace the secret lives in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Secret", Placeholder: "The secret to read", Required: true},
	{Name: "decode", Type: core.ConnectionTypeBoolean, Label: "Decode Values", Placeholder: "Base64-decode the values into `decoded_data`"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Secret Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Secret"},
	{Name: "decoded_data", Type: core.ConnectionTypeObject, Label: "Decoded Data"},
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

	ctx, cancel := kubernetes.Context()
	defer cancel()

	obj, err := kubernetes.Get(ctx, auth, kubernetes.Secrets, namespace, name)
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	// data is map[key]base64String. Count keys for the summary regardless of
	// whether we decode.
	data, _ := obj["data"].(map[string]interface{})
	keyCount := len(data)

	decoded := map[string]string{}
	var skipped []string
	if kubernetes.BoolInput("decode", inputs) {
		for k, v := range data {
			s, ok := v.(string)
			if !ok {
				skipped = append(skipped, k)
				continue
			}
			raw, decErr := base64.StdEncoding.DecodeString(s)
			if decErr != nil {
				// Not valid base64 — leave the key out, name it in the summary,
				// but do not fail the read of the other keys.
				skipped = append(skipped, k)
				continue
			}
			decoded[k] = string(raw)
		}
	}

	summary := fmt.Sprintf("Read secret %s in namespace %s (%d key(s))", name, namespace, keyCount)
	if kubernetes.BoolInput("decode", inputs) {
		summary = fmt.Sprintf("Read secret %s in namespace %s — decoded %d of %d key(s)", name, namespace, len(decoded), keyCount)
		if len(skipped) > 0 {
			sort.Strings(skipped)
			summary += fmt.Sprintf("; left %d key(s) out (not valid base64): %s", len(skipped), strings.Join(skipped, ", "))
		}
	}

	out := kubernetes.ObjectResult(obj, summary)
	out["decoded_data"] = decoded
	if kubernetes.ObjectName(obj) == "" {
		out["id"] = name
	}
	return out, nil
}
