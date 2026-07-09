package marketing_sendgrid_asm_suppression_delete

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: Delete Group Suppression"
	Description  = "Remove an email address from a SendGrid unsubscribe (ASM) group so it resumes receiving email assigned to that group."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+trash"
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
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Unsubscribe Group", Placeholder: "The unsubscribe (ASM) group to remove the address from", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "recipient@example.com — the address to remove from the group", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Email"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
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
	email, err := sendgrid.RequiredString("email", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}

	_, _, _, err = sendgrid.Do(auth, http.MethodDelete, "/v3/asm/groups/"+groupID+"/suppressions/"+url.PathEscape(email), nil, nil)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	return sendgrid.SuccessResult(email, fmt.Sprintf("Removed %s from unsubscribe group %s", email, groupID)), nil
}
