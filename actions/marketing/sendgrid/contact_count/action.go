package marketing_sendgrid_contact_count

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: Count Contacts"
	Description  = "Return the total number of marketing contacts in your SendGrid account, along with the billable contact count."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+hashtag"
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
	{Name: "id", Type: core.ConnectionTypeString, Label: "ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Counts"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := sendgrid.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	result, _, _, err := sendgrid.Do(auth, http.MethodGet, "/v3/marketing/contacts/count", nil, nil)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	obj, ok := result.(map[string]interface{})
	if !ok {
		return sendgrid.ErrorResult("unexpected SendGrid count response shape"), nil
	}
	count := 0
	if total, ok := obj["contact_count"].(float64); ok {
		count = int(total)
	}
	return sendgrid.ResourceResult("", obj, fmt.Sprintf("Account has %d contact(s)", count)), nil
}
