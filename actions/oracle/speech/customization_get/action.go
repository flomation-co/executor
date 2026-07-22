// Package oracle_speech_customization_get fetches a single OCI Speech customization by OCID,
// returning its display name, alias, description, compartment and lifecycle state.
package oracle_speech_customization_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	sp "flomation.app/automate/executor/actions/oracle/speech"

	"github.com/oracle/oci-go-sdk/v65/aispeech"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Speech: Get Customization"
	Description  = "Fetch a single Speech customization by its OCID — its display name, alias, description and lifecycle state."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+microphone"
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
	{Name: "customization_ocid", Type: core.ConnectionTypeString, Label: "Customization OCID", Placeholder: "ocid1.aispeechcustomization.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "customization", Type: core.ConnectionTypeObject, Label: "Customization"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Customization OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := sp.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	customizationID, err := sp.RequiredString("customization_ocid", inputs)
	if err != nil {
		return sp.ErrorResult(err.Error()), nil
	}

	resp, err := client.GetCustomization(sp.Context(), aispeech.GetCustomizationRequest{CustomizationId: &customizationID})
	if err != nil {
		return sp.ErrorResult(auth.OCIError(err)), nil
	}
	customization := sp.SummariseCustomization(&resp.Customization)
	return sp.Result(fmt.Sprintf("Customization %q (%s)", customization["display_name"], customization["lifecycle_state"]), map[string]interface{}{
		"customization": customization, "id": customization["id"], "lifecycle_state": customization["lifecycle_state"],
	}), nil
}
