package trello_board_member_add

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	trello "flomation.app/automate/executor/actions/trello"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Add Board Member"
	Description  = "Add an existing Trello member to a board and set their membership type (normal, admin, or observer)."
	Website      = "https://www.flomation.co"
	Icon         = "trello+user-plus"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Your Trello API key", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Trello API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board to add the member to", Required: true},
	{Name: "member_id", Type: core.ConnectionTypeString, Label: "Member", Placeholder: "The member to add", Required: true},
	{Name: "type", Type: core.ConnectionTypeString, Label: "Member Type", Placeholder: "The member's role on the board", Required: true, Options: []core.ConnectionOption{
		{Name: "Normal", Value: "normal"},
		{Name: "Admin", Value: "admin"},
		{Name: "Observer", Value: "observer"},
	}},
	{Name: "allow_billable_guest", Type: core.ConnectionTypeBoolean, Label: "Allow Billable Guest", Placeholder: "Optionally allow adding a billable guest without a confirmation"},
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
	memberID, err := trello.RequiredString("member_id", inputs)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	memberType, err := trello.RequiredString("type", inputs)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	params := url.Values{}
	params.Set("type", memberType)
	trello.SetBoolIfSet(params, inputs, "allowBillableGuest", "allow_billable_guest")
	obj, err := trello.WriteObject(auth, http.MethodPut, "/boards/"+url.PathEscape(boardID)+"/members/"+url.PathEscape(memberID), params)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	return trello.ResourceResult(obj, "Added member "+memberID+" to board "+boardID), nil
}
