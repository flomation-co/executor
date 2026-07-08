package helpdesk_intercom_team_get

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Get Team"
	Description  = "Retrieve a single team by ID, including the admins in it."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+eye"
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
	{Name: "team_id", Type: core.ConnectionTypeString, Label: "Team", Placeholder: "The team to look up", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Team ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Team"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := intercom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	teamID, err := intercom.RequiredString("team_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	obj, err := intercom.GetObject(auth, "/teams/"+url.PathEscape(teamID), nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	label, _ := obj["name"].(string)
	if label == "" {
		label = teamID
	}
	return intercom.ResourceResult(obj, fmt.Sprintf("Retrieved team %s", label)), nil
}
