package trello_card_comment_update

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	trello "flomation.app/automate/executor/actions/trello"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Update Comment"
	Description  = "Change the text of an existing comment on a Trello card."
	Website      = "https://www.flomation.co"
	Icon         = "trello+pen"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Your Trello API key", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Trello API token", Required: true},
	{Name: "card_id", Type: core.ConnectionTypeString, Label: "Card ID", Placeholder: "The card the comment belongs to", Required: true},
	{Name: "comment_id", Type: core.ConnectionTypeString, Label: "Comment ID", Placeholder: "The ID of the comment to update", Required: true},
	{Name: "text", Type: core.ConnectionTypeText, Label: "Comment", Placeholder: "The new comment text", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Comment ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Comment"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := trello.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	cardID, err := trello.RequiredString("card_id", inputs)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	commentID, err := trello.RequiredString("comment_id", inputs)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	text, err := trello.RequiredString("text", inputs)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	params := url.Values{}
	params.Set("text", text)
	obj, err := trello.WriteObject(auth, http.MethodPut, "/cards/"+url.PathEscape(cardID)+"/actions/"+url.PathEscape(commentID)+"/comments", params)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	return trello.ResourceResult(obj, "Updated comment "+commentID), nil
}
