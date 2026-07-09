package marketing_sendgrid_asm_suppression_list

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
	Name         = "SendGrid: List Group Suppressions"
	Description  = "Retrieve every email address suppressed in a SendGrid unsubscribe (ASM) group. Each result is an object with an email field, ready to loop over. Note: SendGrid applies new suppressions asynchronously, so a just-added address can take a few minutes to appear here."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+list"
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
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Unsubscribe Group", Placeholder: "The unsubscribe (ASM) group whose suppressed addresses to list", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Suppressed Emails"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
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

	result, _, _, err := sendgrid.Do(auth, http.MethodGet, "/v3/asm/groups/"+groupID+"/suppressions", nil, nil)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	items, ok := result.([]interface{})
	if !ok {
		return sendgrid.ErrorResult("unexpected SendGrid response shape for group suppressions"), nil
	}
	// The endpoint answers a bare array of plain email STRINGS — wrap each as
	// {"email": ...} so the results feed a Loop node like every other list.
	wrapped := make([]interface{}, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			wrapped = append(wrapped, map[string]interface{}{"email": s})
			continue
		}
		wrapped = append(wrapped, item)
	}
	return sendgrid.ListResult(wrapped, len(wrapped), fmt.Sprintf("Retrieved %d suppressed email(s) in unsubscribe group %s", len(wrapped), groupID)), nil
}
