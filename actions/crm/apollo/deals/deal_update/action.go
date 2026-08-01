package deal_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Deal: Update"
	Description  = "Update an existing Apollo deal by ID. Only supplied fields change."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+pen"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey}", Required: true},
	{Name: "opportunity_id", Type: core.ConnectionTypeString, Label: "Deal ID", Placeholder: "The Apollo deal (opportunity) ID to update", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Deal Name", Placeholder: "Analytical Engines — Enterprise"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Owner ID", Placeholder: "Apollo user ID of the deal owner"},
	{Name: "amount", Type: core.ConnectionTypeString, Label: "Amount", Placeholder: "50000 (deal value, major units)"},
	{Name: "opportunity_stage_id", Type: core.ConnectionTypeString, Label: "Stage ID", Placeholder: "Apollo deal stage ID"},
	{Name: "closed_date", Type: core.ConnectionTypeString, Label: "Close Date", Placeholder: "2026-09-30"},
	{Name: "is_closed", Type: core.ConnectionTypeBoolean, Label: "Is Closed"},
	{Name: "is_won", Type: core.ConnectionTypeBoolean, Label: "Is Won"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Additional Fields (JSON)", Placeholder: `{"…":"…"}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Deal ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Deal"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := apollo_common.GetAPIKey(inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}
	id, err := apollo_common.RequiredString("opportunity_id", inputs)
	if err != nil {
		return apollo_common.ErrorResult("a deal ID is required"), nil
	}

	body := map[string]interface{}{}
	apollo_common.SetString(body, "name", "name", inputs)
	apollo_common.SetString(body, "owner_id", "owner_id", inputs)
	apollo_common.SetNumberValue(body, "amount", "amount", inputs)
	apollo_common.SetString(body, "opportunity_stage_id", "opportunity_stage_id", inputs)
	apollo_common.SetString(body, "closed_date", "closed_date", inputs)
	apollo_common.SetBool(body, "is_closed", "is_closed", inputs)
	apollo_common.SetBool(body, "is_won", "is_won", inputs)

	extra, err := apollo_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}
	apollo_common.MergeFields(body, extra)

	resp, err := apollo_common.NewClient(apiKey).Patch(flow, "/opportunities/"+id, body)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	deal := apollo_common.Obj(resp, "opportunity")
	if deal == nil {
		return apollo_common.ErrorResult("deal was not updated"), nil
	}
	return apollo_common.ObjectResult(id, deal, fmt.Sprintf("Updated deal %s", id)), nil
}
