// Package oracle_documentunderstanding_model_get fetches a single Document Understanding custom model
// by OCID, returning its type, version, training details and lifecycle state.
package oracle_documentunderstanding_model_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	du "flomation.app/automate/executor/actions/oracle/documentunderstanding"

	"github.com/oracle/oci-go-sdk/v65/aidocument"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Document Understanding: Get Model"
	Description  = "Fetch a single Document Understanding custom model by its OCID — its type, version, training details and lifecycle state."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+image"
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
	{Name: "model_ocid", Type: core.ConnectionTypeString, Label: "Model OCID", Placeholder: "ocid1.aidocumentmodel.oc1..aaaa…", Required: true},
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
	auth, client, errResult := du.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	modelID, err := du.RequiredString("model_ocid", inputs)
	if err != nil {
		return du.ErrorResult(err.Error()), nil
	}

	resp, err := client.GetModel(du.Context(), aidocument.GetModelRequest{ModelId: &modelID})
	if err != nil {
		return du.ErrorResult(auth.OCIError(err)), nil
	}
	model := du.SummariseModel(&resp.Model)
	return du.Result(fmt.Sprintf("Model %q (%s)", model["display_name"], model["lifecycle_state"]), map[string]interface{}{
		"model": model, "id": model["id"], "lifecycle_state": model["lifecycle_state"],
	}), nil
}
