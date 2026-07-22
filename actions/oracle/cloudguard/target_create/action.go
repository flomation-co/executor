// Package oracle_cloudguard_target_create creates a Cloud Guard target: the scope (a compartment,
// or an ERP/HCM Cloud instance) that Cloud Guard monitors, applying its detector and responder
// recipes to the resources found beneath it.
package oracle_cloudguard_target_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	cg "flomation.app/automate/executor/actions/oracle/cloudguard"

	"github.com/oracle/oci-go-sdk/v65/cloudguard"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Cloud Guard: Create Target"
	Description  = "Create a Cloud Guard target — the compartment (or ERP/HCM Cloud instance) whose resources Cloud Guard monitors against its detector and responder recipes."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… — where the target is created", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the target", Required: true},
	{Name: "target_resource_type", Type: core.ConnectionTypeString, Label: "Target Resource Type", Placeholder: "The kind of resource the target monitors", Required: true, Options: []core.ConnectionOption{
		{Name: "Compartment", Value: "COMPARTMENT"},
		{Name: "ERP Cloud", Value: "ERPCLOUD"},
		{Name: "HCM Cloud", Value: "HCMCLOUD"},
	}},
	{Name: "target_resource_ocid", Type: core.ConnectionTypeString, Label: "Target Resource OCID", Placeholder: "OCID of the compartment / ERP / HCM instance to monitor", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Optional"},
	{Name: "freeform_tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: "{\"env\":\"prod\"} (optional)"},
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
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return cg.ErrorResult(err.Error()), nil
	}
	name, err := cg.RequiredString("display_name", inputs)
	if err != nil {
		return cg.ErrorResult(err.Error()), nil
	}
	resourceType, err := cg.RequiredString("target_resource_type", inputs)
	if err != nil {
		return cg.ErrorResult(err.Error()), nil
	}
	if _, ok := cloudguard.GetMappingTargetResourceTypeEnum(resourceType); !ok {
		return cg.ErrorResult("target resource type must be COMPARTMENT, ERPCLOUD or HCMCLOUD"), nil
	}
	resourceID, err := cg.RequiredString("target_resource_ocid", inputs)
	if err != nil {
		return cg.ErrorResult(err.Error()), nil
	}

	details := cloudguard.CreateTargetDetails{
		DisplayName:        &name,
		CompartmentId:      &compartment,
		TargetResourceType: cloudguard.TargetResourceTypeEnum(resourceType),
		TargetResourceId:   &resourceID,
	}
	if d := cg.OptionalString("description", inputs); d != "" {
		details.Description = &d
	}
	if tags, err := cg.FreeformTags("freeform_tags", inputs); err != nil {
		return cg.ErrorResult(err.Error()), nil
	} else if tags != nil {
		details.FreeformTags = tags
	}

	resp, err := client.CreateTarget(cg.Context(), cloudguard.CreateTargetRequest{CreateTargetDetails: details})
	if err != nil {
		return cg.ErrorResult(auth.OCIError(err)), nil
	}
	target := cg.SummariseTarget(&resp.Target)
	return cg.Result(fmt.Sprintf("Created Cloud Guard target %q (%s)", target["display_name"], target["lifecycle_state"]), map[string]interface{}{
		"target": target, "id": target["id"], "lifecycle_state": target["lifecycle_state"],
	}), nil
}
