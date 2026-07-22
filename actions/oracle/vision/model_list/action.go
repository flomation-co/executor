// Package oracle_vision_model_list lists the custom Vision models in a compartment, optionally
// filtered by project, exact display name, or lifecycle state. Walks pagination up to a safe cap.
package oracle_vision_model_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	vis "flomation.app/automate/executor/actions/oracle/vision"

	"github.com/oracle/oci-go-sdk/v65/aivision"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vision: List Models"
	Description  = "List the custom Vision models in a compartment. Optionally filter by project, exact display name, or lifecycle state. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+eye"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project OCID Filter", Placeholder: "Only models in this project (optional)"},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name Filter", Placeholder: "Only models with this exact name (optional)"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State", Placeholder: "Only models in this state (optional)", Options: []core.ConnectionOption{
		{Name: "Creating", Value: "CREATING"},
		{Name: "Updating", Value: "UPDATING"},
		{Name: "Active", Value: "ACTIVE"},
		{Name: "Deleting", Value: "DELETING"},
		{Name: "Deleted", Value: "DELETED"},
		{Name: "Failed", Value: "FAILED"},
	}},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Max items per page (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "models", Type: core.ConnectionTypeObject, Label: "Models"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := vis.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return vis.ErrorResult(err.Error()), nil
	}
	req := aivision.ListModelsRequest{CompartmentId: &compartment}
	if projectID := vis.OptionalString("project_id", inputs); projectID != "" {
		req.ProjectId = &projectID
	}
	if name := vis.OptionalString("display_name", inputs); name != "" {
		req.DisplayName = &name
	}
	if state := vis.OptionalString("lifecycle_state", inputs); state != "" {
		req.LifecycleState = aivision.ModelLifecycleStateEnum(state)
	}
	limit, ok, err := vis.OptionalInt("limit", inputs)
	if err != nil {
		return vis.ErrorResult(err.Error()), nil
	}
	if ok {
		req.Limit = &limit
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= vis.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListModels(vis.Context(), req)
		if err != nil {
			return vis.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, vis.SummariseModelSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return vis.Result(fmt.Sprintf("Found %d model(s)", len(out)), map[string]interface{}{
		"models": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
