package marketing_sendgrid_asm_group_update

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
	Name         = "SendGrid: Update Unsubscribe Group"
	Description  = "Update an unsubscribe (ASM) group's name, description, or default setting in SendGrid. Only the fields you provide are changed."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+pencil"
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
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Unsubscribe Group", Placeholder: "The unsubscribe (ASM) group to update", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "New group name — up to 30 characters, unique in your account"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "New description — what recipients see when choosing what to unsubscribe from, up to 100 characters"},
	{Name: "is_default", Type: core.ConnectionTypeBoolean, Label: "Default Group", Placeholder: "Tick to make this the default group for emails sent without an unsubscribe group"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `Any other SendGrid field to include in the request body`},
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

	body := map[string]interface{}{}
	sendgrid.SetIfPresent(body, inputs, "name", "name")
	sendgrid.SetIfPresent(body, inputs, "description", "description")
	sendgrid.SetBoolIfSet(body, inputs, "is_default", "is_default")
	if err := sendgrid.MergeAdditionalFields(body, inputs); err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	if len(body) == 0 {
		return sendgrid.ErrorResult("provide a Name, Description, or Default Group setting to update"), nil
	}

	// Quirk: this PATCH answers 201 rather than 200 — any 2xx is success.
	result, _, _, err := sendgrid.Do(auth, http.MethodPatch, "/v3/asm/groups/"+groupID, nil, body)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	obj, _ := result.(map[string]interface{})
	return sendgrid.ResourceResult(groupID, obj, fmt.Sprintf("Updated unsubscribe group %s", groupID)), nil
}
