// Package oracle_objectstorage_namespace_get returns the tenancy's Object Storage
// namespace — the top-level container name every bucket lives under. The other
// actions resolve it automatically; this exposes it directly when a flow needs it.
package oracle_objectstorage_namespace_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	os "flomation.app/automate/executor/actions/oracle/objectstorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Object Storage: Get Namespace"
	Description  = "Return the tenancy's Object Storage namespace — the top-level container every bucket lives under. The other actions resolve it automatically; use this when a flow needs the namespace value directly."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+circle-info"
	Date         = "21/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := os.GetAuth(inputs)
	if err != nil {
		return os.ErrorResult(err.Error()), nil
	}
	client, err := auth.Client()
	if err != nil {
		return os.ErrorResult(auth.OCIError(err)), nil
	}
	ns, err := auth.Namespace(os.Context(), client)
	if err != nil {
		return os.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Tenancy namespace is %s", ns),
		"namespace":   ns,
		"success":     true,
	}, nil
}
