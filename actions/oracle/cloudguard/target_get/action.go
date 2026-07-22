// Package oracle_cloudguard_target_get fetches a single Cloud Guard target by OCID, returning the
// monitored resource, attached recipes and lifecycle state.
package oracle_cloudguard_target_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	cg "flomation.app/automate/executor/actions/oracle/cloudguard"

	"github.com/oracle/oci-go-sdk/v65/cloudguard"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Cloud Guard: Get Target"
	Description  = "Fetch a single Cloud Guard target by its OCID — its monitored resource, recipes and lifecycle state."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+shield-halved"
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
	{Name: "target_ocid", Type: core.ConnectionTypeString, Label: "Target OCID", Placeholder: "ocid1.cloudguardtarget.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "target", Type: core.ConnectionTypeObject, Label: "Target"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Target OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := cg.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	targetID, err := cg.RequiredString("target_ocid", inputs)
	if err != nil {
		return cg.ErrorResult(err.Error()), nil
	}

	resp, err := client.GetTarget(cg.Context(), cloudguard.GetTargetRequest{TargetId: &targetID})
	if err != nil {
		return cg.ErrorResult(auth.OCIError(err)), nil
	}
	target := cg.SummariseTarget(&resp.Target)
	return cg.Result(fmt.Sprintf("Target %q (%s)", target["display_name"], target["lifecycle_state"]), map[string]interface{}{
		"target": target, "id": target["id"], "lifecycle_state": target["lifecycle_state"],
	}), nil
}
