package marketing_sendgrid_asm_group_list

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: List Unsubscribe Groups"
	Description  = "Retrieve all unsubscribe (ASM) groups in your SendGrid account, including each group's name, description, and unsubscribe count."
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
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Unsubscribe Groups"},
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

	// The endpoint answers with a TOP-LEVEL ARRAY of every group — no
	// envelope, no pagination.
	result, _, _, err := sendgrid.Do(auth, http.MethodGet, "/v3/asm/groups", nil, nil)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	items, ok := result.([]interface{})
	if !ok {
		return sendgrid.ErrorResult("unexpected SendGrid response shape for unsubscribe groups"), nil
	}
	return sendgrid.ListResult(items, len(items), fmt.Sprintf("Retrieved %d unsubscribe group(s)", len(items))), nil
}
