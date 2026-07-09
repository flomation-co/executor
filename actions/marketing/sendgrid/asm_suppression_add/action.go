package marketing_sendgrid_asm_suppression_add

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
	Name         = "SendGrid: Add Group Suppressions"
	Description  = "Add one or more email addresses to a SendGrid unsubscribe (ASM) group so they stop receiving email assigned to that group. Separate multiple addresses with commas."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+ban"
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
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Unsubscribe Group", Placeholder: "The unsubscribe (ASM) group to add the addresses to", Required: true},
	{Name: "emails", Type: core.ConnectionTypeString, Label: "Emails", Placeholder: "recipient@example.com — separate multiple addresses with commas", Required: true},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `Any other SendGrid field to include in the request body`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Group ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Added Emails"},
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
	emailsRaw, err := sendgrid.RequiredString("emails", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	// RequiredString guarantees non-empty input, but a value of only commas
	// and spaces still splits to nothing.
	emails := sendgrid.SplitCSV(emailsRaw)
	if len(emails) == 0 {
		return sendgrid.ErrorResult("emails must contain at least one address"), nil
	}

	body := map[string]interface{}{"recipient_emails": emails}
	if err := sendgrid.MergeAdditionalFields(body, inputs); err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}

	result, _, _, err := sendgrid.Do(auth, http.MethodPost, "/v3/asm/groups/"+groupID+"/suppressions", nil, body)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	obj, _ := result.(map[string]interface{})
	return sendgrid.ResourceResult(groupID, obj, fmt.Sprintf("Added %d email(s) to unsubscribe group %s", len(emails), groupID)), nil
}
