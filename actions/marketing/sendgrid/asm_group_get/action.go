package marketing_sendgrid_asm_group_get

import (
	"fmt"
	"net/http"
	"strconv"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: Get Unsubscribe Group"
	Description  = "Retrieve a single unsubscribe (ASM) group from SendGrid, including its name, description, and how many recipients have unsubscribed from it."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+eye"
	Date         = "09/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "SendGrid API key (SendGrid → Settings → API Keys), e.g. ${secrets.sendgrid_api}", Required: true},
	{
		Name:  "region",
		Type:  core.ConnectionTypeString,
		Label: "Region",
		Options: []core.ConnectionOption{
			{Name: "Global", Value: ""},
			{Name: "EU (data residency)", Value: "eu"},
		},
		Placeholder: "Global unless your account uses an EU regional subuser — the EU host has no Marketing endpoints (contacts, lists, segments)",
	},
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Unsubscribe Group", Placeholder: "The unsubscribe (ASM) group to retrieve", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Group ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Unsubscribe Group"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := sendgrid.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	groupID, err := sendgrid.RequiredString("group_id", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	if _, convErr := strconv.Atoi(groupID); convErr != nil {
		return sendgrid.ErrorResult(fmt.Sprintf("group_id must be a whole number (got %q)", groupID)), nil
	}

	result, _, _, err := sendgrid.Do(auth, http.MethodGet, "/v3/asm/groups/"+groupID, nil, nil)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	obj, _ := result.(map[string]interface{})
	return sendgrid.ResourceResult(groupID, obj, fmt.Sprintf("Retrieved unsubscribe group %s", groupID)), nil
}
