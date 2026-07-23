// Package oracle_vision_model_get fetches a single OCI Vision custom model by its OCID, returning
// its display name, type, version, project, lifecycle state and creation time.
package oracle_vision_model_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	vis "flomation.app/automate/executor/actions/oracle/vision"

	"github.com/oracle/oci-go-sdk/v65/aivision"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vision: Get Model"
	Description  = "Fetch a single OCI Vision custom model by its OCID — its type, version, project and lifecycle state."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+eye"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "model_ocid", Type: core.ConnectionTypeString, Label: "Model OCID", Placeholder: "ocid1.aivisionmodel.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "model", Type: core.ConnectionTypeObject, Label: "Model"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Model OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := vis.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	modelID, err := vis.RequiredString("model_ocid", inputs)
	if err != nil {
		return vis.ErrorResult(err.Error()), nil
	}

	resp, err := client.GetModel(vis.Context(), aivision.GetModelRequest{ModelId: &modelID})
	if err != nil {
		return vis.ErrorResult(auth.OCIError(err)), nil
	}
	model := vis.SummariseModel(&resp.Model)
	return vis.Result(fmt.Sprintf("Model %q (%s)", model["display_name"], model["lifecycle_state"]), map[string]interface{}{
		"model": model, "id": model["id"], "lifecycle_state": model["lifecycle_state"],
	}), nil
}
