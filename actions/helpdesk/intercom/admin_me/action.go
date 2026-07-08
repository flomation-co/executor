package helpdesk_intercom_admin_me

import (
	"fmt"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Get Current Admin"
	Description  = "Look up the admin (teammate) your access token belongs to — an easy way to test the connection and find your own admin ID."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+user"
	Date         = "08/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Intercom access token (Developer Hub → Authentication)", Required: true},
	{
		Name:  "region",
		Type:  core.ConnectionTypeString,
		Label: "Region",
		Options: []core.ConnectionOption{
			{Name: "US (default)", Value: "us"},
			{Name: "Europe", Value: "eu"},
			{Name: "Australia", Value: "au"},
		},
	},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Admin ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Admin"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := intercom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	obj, err := intercom.GetObject(auth, "/me", nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	name, _ := obj["name"].(string)
	email, _ := obj["email"].(string)
	summary := "Retrieved the current admin"
	switch {
	case name != "" && email != "":
		summary = fmt.Sprintf("Authenticated as %s (%s)", name, email)
	case name != "":
		summary = fmt.Sprintf("Authenticated as %s", name)
	case email != "":
		summary = fmt.Sprintf("Authenticated as %s", email)
	}
	return intercom.ResourceResult(obj, summary), nil
}
