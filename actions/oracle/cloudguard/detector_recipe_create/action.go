// Package oracle_cloudguard_detector_recipe_create clones an Oracle-managed detector recipe into
// a user-owned detector recipe in a compartment, so its rules can then be tuned for a tenancy.
package oracle_cloudguard_detector_recipe_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	cg "flomation.app/automate/executor/actions/oracle/cloudguard"

	"github.com/oracle/oci-go-sdk/v65/cloudguard"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Cloud Guard: Create Detector Recipe"
	Description  = "Create a detector recipe by cloning an Oracle-managed source recipe into a compartment, ready for its rules to be tuned."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+shield-halved"
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
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the new detector recipe", Required: true},
	{Name: "source_detector_recipe_ocid", Type: core.ConnectionTypeString, Label: "Source Detector Recipe OCID", Placeholder: "The Oracle-managed detector recipe to clone", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Optional"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "recipe", Type: core.ConnectionTypeObject, Label: "Detector Recipe"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Detector Recipe OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := cg.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return cg.ErrorResult(err.Error()), nil
	}
	name, err := cg.RequiredString("display_name", inputs)
	if err != nil {
		return cg.ErrorResult(err.Error()), nil
	}
	source, err := cg.RequiredString("source_detector_recipe_ocid", inputs)
	if err != nil {
		return cg.ErrorResult(err.Error()), nil
	}

	details := cloudguard.CreateDetectorRecipeDetails{
		DisplayName:            &name,
		CompartmentId:          &compartment,
		SourceDetectorRecipeId: &source,
	}
	if d := cg.OptionalString("description", inputs); d != "" {
		details.Description = &d
	}

	resp, err := client.CreateDetectorRecipe(cg.Context(), cloudguard.CreateDetectorRecipeRequest{CreateDetectorRecipeDetails: details})
	if err != nil {
		return cg.ErrorResult(auth.OCIError(err)), nil
	}
	recipe := cg.SummariseDetectorRecipe(&resp.DetectorRecipe)
	return cg.Result(fmt.Sprintf("Created detector recipe %q (%s)", recipe["display_name"], recipe["lifecycle_state"]), map[string]interface{}{
		"recipe": recipe, "id": recipe["id"], "lifecycle_state": recipe["lifecycle_state"],
	}), nil
}
