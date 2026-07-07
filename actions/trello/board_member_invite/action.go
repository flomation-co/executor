package trello_board_member_invite

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	trello "flomation.app/automate/executor/actions/trello"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Invite Board Member by Email"
	Description  = "Invite someone to a Trello board by email address, optionally setting their membership type and full name."
	Website      = "https://www.flomation.co"
	Icon         = "trello+user-plus"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Your Trello API key", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Trello API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board to invite the member to", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "The email address to invite", Required: true},
	{Name: "type", Type: core.ConnectionTypeString, Label: "Member Type", Placeholder: "The member's role on the board", Options: []core.ConnectionOption{
		{Name: "Normal", Value: "normal"},
		{Name: "Admin", Value: "admin"},
		{Name: "Observer", Value: "observer"},
	}},
	{Name: "full_name", Type: core.ConnectionTypeString, Label: "Full Name", Placeholder: "The invitee's full name (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Board ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Board"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := trello.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	boardID, err := trello.RequiredString("board_id", inputs)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	email, err := trello.RequiredString("email", inputs)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	params := url.Values{}
	params.Set("email", email)
	trello.SetIfPresent(params, inputs, "type", "type")
	trello.SetIfPresent(params, inputs, "fullName", "full_name")
	obj, err := trello.WriteObject(auth, http.MethodPut, "/boards/"+url.PathEscape(boardID)+"/members", params)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	return trello.ResourceResult(obj, "Invited "+email+" to board "+boardID), nil
}
