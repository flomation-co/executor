// Package oracle_cloudguard_target_update applies a partial update to a Cloud Guard target:
// only the display name you supply is changed; a blank field is left as-is, and the target's
// existing detector/responder recipes are preserved.
package oracle_cloudguard_target_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	cg "flomation.app/automate/executor/actions/oracle/cloudguard"

	"github.com/oracle/oci-go-sdk/v65/cloudguard"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Cloud Guard: Update Target"
	Description  = "Partially update a Cloud Guard target — change only the display name you supply; a blank field is left unchanged and the target's existing detector and responder recipes are preserved."
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
	{Name: "target_ocid", Type: core.ConnectionTypeString, Label: "Target OCID", Placeholder: "ocid1.cloudguardtarget.oc1..aaaa… — the target to update", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name (leave blank to keep unchanged)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "target", Type: core.ConnectionTypeObject, Label: "Target"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Target OCID"},
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

	// Partial update: only carry the display name when the operator supplies one. The recipe
	// slices are left nil so the target keeps its existing detector/responder recipes.
	details := cloudguard.UpdateTargetDetails{}
	if v := cg.OptionalString("display_name", inputs); v != "" {
		details.DisplayName = &v
	}

	resp, err := client.UpdateTarget(cg.Context(), cloudguard.UpdateTargetRequest{
		TargetId:            &targetID,
		UpdateTargetDetails: details,
	})
	if err != nil {
		return cg.ErrorResult(auth.OCIError(err)), nil
	}
	target := cg.SummariseTarget(&resp.Target)
	return cg.Result(fmt.Sprintf("Updated target %q (%s)", target["display_name"], target["lifecycle_state"]), map[string]interface{}{
		"target": target,
		"id":     target["id"],
	}), nil
}
