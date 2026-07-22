// Package oracle_cloudguard_detector_recipe_update applies a partial update to a Cloud Guard
// detector recipe: only the display name and description you supply are changed; blank fields are
// left as-is, and the recipe's detector rules are preserved.
package oracle_cloudguard_detector_recipe_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	cg "flomation.app/automate/executor/actions/oracle/cloudguard"

	"github.com/oracle/oci-go-sdk/v65/cloudguard"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Cloud Guard: Update Detector Recipe"
	Description  = "Partially update a Cloud Guard detector recipe — change only the display name or description you supply; blank fields are left unchanged and the recipe's detector rules are preserved."
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
	{Name: "detector_recipe_ocid", Type: core.ConnectionTypeString, Label: "Detector Recipe OCID", Placeholder: "ocid1.cloudguarddetectorrecipe.oc1..aaaa… — the recipe to update", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name (leave blank to keep unchanged)"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "New description (leave blank to keep unchanged)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "recipe", Type: core.ConnectionTypeObject, Label: "Detector Recipe"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Detector Recipe OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := cg.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	recipeID, err := cg.RequiredString("detector_recipe_ocid", inputs)
	if err != nil {
		return cg.ErrorResult(err.Error()), nil
	}

	// Partial update: only carry the fields the operator actually supplied. DetectorRules is left
	// nil so the recipe keeps its existing rules.
	details := cloudguard.UpdateDetectorRecipeDetails{}
	if v := cg.OptionalString("display_name", inputs); v != "" {
		details.DisplayName = &v
	}
	if v := cg.OptionalString("description", inputs); v != "" {
		details.Description = &v
	}

	resp, err := client.UpdateDetectorRecipe(cg.Context(), cloudguard.UpdateDetectorRecipeRequest{
		DetectorRecipeId:            &recipeID,
		UpdateDetectorRecipeDetails: details,
	})
	if err != nil {
		return cg.ErrorResult(auth.OCIError(err)), nil
	}
	recipe := cg.SummariseDetectorRecipe(&resp.DetectorRecipe)
	return cg.Result(fmt.Sprintf("Updated detector recipe %q (%s)", recipe["display_name"], recipe["lifecycle_state"]), map[string]interface{}{
		"recipe": recipe, "id": recipe["id"],
	}), nil
}
