package sequence_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Sequence: Create"
	Description  = "Create an Apollo sequence (emailer campaign). Needs a name. Master key required."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+plus"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey} (master key)", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Sequence Name", Placeholder: "Q3 Enterprise Outbound", Required: true},
	{Name: "permissions", Type: core.ConnectionTypeString, Label: "Visibility", Placeholder: "team_can_use", Options: []core.ConnectionOption{
		{Name: "Team can use", Value: "team_can_use"},
		{Name: "Team can view", Value: "team_can_view"},
		{Name: "Private", Value: "private"},
	}},
	{Name: "active", Type: core.ConnectionTypeBoolean, Label: "Active", Placeholder: "Start the sequence active (default off)"},
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "Owner (User ID)", Placeholder: "Apollo user ID of the owner"},
	{Name: "label_names", Type: core.ConnectionTypeString, Label: "Folder/Label Names", Placeholder: "Comma-separated"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Additional Fields (JSON)", Placeholder: `{"emailer_steps":[…],"max_emails_per_day":50}`},
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
	if _, err := apollo_common.RequiredString("name", inputs); err != nil {
		return apollo_common.ErrorResult("a sequence name is required"), nil
	}

	body := map[string]interface{}{}
	apollo_common.SetString(body, "name", "name", inputs)
	apollo_common.SetString(body, "permissions", "permissions", inputs)
	apollo_common.SetBool(body, "active", "active", inputs)
	apollo_common.SetString(body, "user_id", "user_id", inputs)
	apollo_common.SetList(body, "label_names", "label_names", inputs)

	extra, err := apollo_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}
	apollo_common.MergeFields(body, extra)

	// Sequence CRUD lives under /sequences (membership/search use /emailer_campaigns).
	resp, err := apollo_common.NewClient(apiKey).Post(flow, "/sequences", body)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	seq := apollo_common.Obj(resp, "emailer_campaign")
	if seq == nil {
		return apollo_common.ErrorResult("sequence was not created"), nil
	}
	return apollo_common.ObjectResult("", seq, fmt.Sprintf("Created sequence %s", apollo_common.IDOf(seq))), nil
}
