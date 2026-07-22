// Package oracle_generativeai_model_get fetches a single Generative AI model by OCID, returning its
// vendor, capabilities, base model and lifecycle state.
package oracle_generativeai_model_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	gai "flomation.app/automate/executor/actions/oracle/generativeai"

	"github.com/oracle/oci-go-sdk/v65/generativeai"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Generative AI: Get Model"
	Description  = "Fetch a single Generative AI model by its OCID — its vendor, version, capabilities, base model and lifecycle state."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+robot"
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
	{Name: "model_ocid", Type: core.ConnectionTypeString, Label: "Model OCID", Placeholder: "ocid1.generativeaimodel.oc1..aaaa…", Required: true},
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
	auth, client, errResult := gai.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	modelID, err := gai.RequiredString("model_ocid", inputs)
	if err != nil {
		return gai.ErrorResult(err.Error()), nil
	}

	resp, err := client.GetModel(gai.Context(), generativeai.GetModelRequest{ModelId: &modelID})
	if err != nil {
		return gai.ErrorResult(auth.OCIError(err)), nil
	}
	model := gai.SummariseModel(&resp.Model)
	return gai.Result(fmt.Sprintf("Model %q (%s)", model["display_name"], model["lifecycle_state"]), map[string]interface{}{
		"model": model, "id": model["id"], "lifecycle_state": model["lifecycle_state"],
	}), nil
}
