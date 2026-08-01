package deal_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Deal: Create"
	Description  = "Create a deal (opportunity) in your Apollo CRM. Returns the deal ID and object."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+plus"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey}", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Deal Name", Placeholder: "Analytical Engines — Enterprise", Required: true},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Owner ID", Placeholder: "Apollo user ID of the deal owner"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Account ID", Placeholder: "Apollo account this deal belongs to"},
	{Name: "amount", Type: core.ConnectionTypeString, Label: "Amount", Placeholder: "50000 (deal value, major units)"},
	{Name: "opportunity_stage_id", Type: core.ConnectionTypeString, Label: "Stage ID", Placeholder: "Apollo deal stage ID"},
	{Name: "closed_date", Type: core.ConnectionTypeString, Label: "Close Date", Placeholder: "2026-09-30"},
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
	if _, err := apollo_common.RequiredString("name", inputs); err != nil {
		return apollo_common.ErrorResult("a deal name is required"), nil
	}

	body := map[string]interface{}{}
	apollo_common.SetString(body, "name", "name", inputs)
	apollo_common.SetString(body, "owner_id", "owner_id", inputs)
	apollo_common.SetString(body, "account_id", "account_id", inputs)
	apollo_common.SetNumberValue(body, "amount", "amount", inputs)
	apollo_common.SetString(body, "opportunity_stage_id", "opportunity_stage_id", inputs)
	apollo_common.SetString(body, "closed_date", "closed_date", inputs)

	extra, err := apollo_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}
	apollo_common.MergeFields(body, extra)

	resp, err := apollo_common.NewClient(apiKey).Post(flow, "/opportunities", body)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	deal := apollo_common.Obj(resp, "opportunity")
	if deal == nil {
		return apollo_common.ErrorResult("deal was not created"), nil
	}
	return apollo_common.ObjectResult("", deal, fmt.Sprintf("Created deal %s", apollo_common.IDOf(deal))), nil
}
