package sequence_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Sequence: Update"
	Description  = "Update an Apollo sequence by ID (name, active, visibility…). Master key required."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+pen"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey} (master key)", Required: true},
	{Name: "sequence_id", Type: core.ConnectionTypeString, Label: "Sequence ID", Placeholder: "The Apollo sequence (emailer campaign) ID", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Sequence Name", Placeholder: "Q3 Enterprise Outbound"},
	{Name: "active", Type: core.ConnectionTypeBoolean, Label: "Active"},
	{Name: "permissions", Type: core.ConnectionTypeString, Label: "Visibility", Placeholder: "team_can_use", Options: []core.ConnectionOption{
		{Name: "Team can use", Value: "team_can_use"},
		{Name: "Team can view", Value: "team_can_view"},
		{Name: "Private", Value: "private"},
	}},
	{Name: "label_names", Type: core.ConnectionTypeString, Label: "Folder/Label Names", Placeholder: "Comma-separated"},
	{Name: "max_emails_per_day", Type: core.ConnectionTypeInteger, Label: "Max Emails Per Day", Placeholder: "50"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Additional Fields (JSON)", Placeholder: `{"emailer_steps":[…]}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Sequence ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Sequence"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := apollo_common.GetAPIKey(inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}
	id, err := apollo_common.RequiredString("sequence_id", inputs)
	if err != nil {
		return apollo_common.ErrorResult("a sequence ID is required"), nil
	}

	body := map[string]interface{}{}
	apollo_common.SetString(body, "name", "name", inputs)
	apollo_common.SetBool(body, "active", "active", inputs)
	apollo_common.SetString(body, "permissions", "permissions", inputs)
	apollo_common.SetList(body, "label_names", "label_names", inputs)
	apollo_common.SetInt(body, "max_emails_per_day", "max_emails_per_day", inputs)

	extra, err := apollo_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}
	apollo_common.MergeFields(body, extra)

	// PUT (not PATCH) under /sequences.
	resp, err := apollo_common.NewClient(apiKey).Request(flow, "PUT", "/sequences/"+id, nil, body)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	seq := apollo_common.Obj(resp, "emailer_campaign")
	if seq == nil {
		return apollo_common.ErrorResult("sequence was not updated"), nil
	}
	return apollo_common.ObjectResult(id, seq, fmt.Sprintf("Updated sequence %s", id)), nil
}
