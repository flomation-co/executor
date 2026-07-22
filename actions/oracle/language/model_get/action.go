// Package oracle_language_model_get fetches a single OCI Language custom model by OCID, returning
// its display name, project, version, language code and lifecycle state.
package oracle_language_model_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	lang "flomation.app/automate/executor/actions/oracle/language"

	"github.com/oracle/oci-go-sdk/v65/ailanguage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Language: Get Model"
	Description  = "Fetch a single OCI Language custom model by its OCID — its display name, project, version and lifecycle state."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+comments"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "model_ocid", Type: core.ConnectionTypeString, Label: "Model OCID", Placeholder: "ocid1.ailanguagemodel.oc1..aaaa…", Required: true},
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
	auth, client, errResult := lang.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	modelID, err := lang.RequiredString("model_ocid", inputs)
	if err != nil {
		return lang.ErrorResult(err.Error()), nil
	}

	resp, err := client.GetModel(lang.Context(), ailanguage.GetModelRequest{ModelId: &modelID})
	if err != nil {
		return lang.ErrorResult(auth.OCIError(err)), nil
	}
	model := lang.SummariseModel(&resp.Model)
	return lang.Result(fmt.Sprintf("Model %q (%s)", model["display_name"], model["lifecycle_state"]), map[string]interface{}{
		"model": model, "id": model["id"], "lifecycle_state": model["lifecycle_state"],
	}), nil
}
