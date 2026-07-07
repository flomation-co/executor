package trello_board_member_remove

import (
	"net/url"

	core "flomation.app/automate/executor"
	trello "flomation.app/automate/executor/actions/trello"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Remove Board Member"
	Description  = "Remove a member from a Trello board."
	Website      = "https://www.flomation.co"
	Icon         = "trello+user-minus"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Your Trello API key", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Trello API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board to remove the member from", Required: true},
	{Name: "member_id", Type: core.ConnectionTypeString, Label: "Member", Placeholder: "The member to remove", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Board ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
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
	if err := trello.DeleteOK(auth, "/boards/"+url.PathEscape(boardID)+"/members/"+url.PathEscape(memberID), url.Values{}); err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	return trello.SuccessResult(boardID, nil, "Removed member "+memberID+" from board "+boardID), nil
}
