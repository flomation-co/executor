package trello_label_add_to_card

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	trello "flomation.app/automate/executor/actions/trello"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Add Label to Card"
	Description  = "Attach an existing label to a Trello card."
	Website      = "https://www.flomation.co"
	Icon         = "trello+plus"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Your Trello API key", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Trello API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board that owns the label (used to load the label picker)"},
	{Name: "card_id", Type: core.ConnectionTypeString, Label: "Card ID", Placeholder: "The card to add the label to", Required: true},
	{Name: "label_id", Type: core.ConnectionTypeString, Label: "Label", Placeholder: "The label to add", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Card ID"},
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
	cardID, err := trello.RequiredString("card_id", inputs)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	labelID, err := trello.RequiredString("label_id", inputs)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	params := url.Values{}
	params.Set("value", labelID)
	// POST .../idLabels returns the card's new array of label ids, not an object,
	// so use the status-only helper (the response body is not needed here).
	if err := trello.WriteOK(auth, http.MethodPost, "/cards/"+url.PathEscape(cardID)+"/idLabels", params); err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	return trello.SuccessResult(cardID, map[string]interface{}{"card_id": cardID, "label_id": labelID}, "Added label "+labelID+" to card "+cardID), nil
}
