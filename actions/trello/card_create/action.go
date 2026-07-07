package trello_card_create

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	trello "flomation.app/automate/executor/actions/trello"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Create Card"
	Description  = "Create a new card in a Trello list. Pick a board to load its lists, choose the list, and give the card a name plus optional description, due date, members, and labels."
	Website      = "https://www.flomation.co"
	Icon         = "trello+plus"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

// board_id is a helper input: it scopes the List picker (via the api's live
// dropdown) but is NOT sent to Trello — card creation is keyed on the list id.
var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Your Trello API key", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Trello API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board that owns the list (used to load the list picker)"},
	{Name: "list_id", Type: core.ConnectionTypeString, Label: "List", Placeholder: "The list to create the card in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "The name of the card", Required: true},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "An optional description for the card"},
	{Name: "due", Type: core.ConnectionTypeDateTime, Label: "Due Date", Placeholder: "An optional due date for the card"},
	{Name: "position", Type: core.ConnectionTypeString, Label: "Position", Placeholder: "top, bottom, or a positive number"},
	{Name: "member_ids", Type: core.ConnectionTypeString, Label: "Member IDs", Placeholder: "Comma-separated member IDs to assign"},
	{Name: "label_ids", Type: core.ConnectionTypeString, Label: "Label IDs", Placeholder: "Comma-separated label IDs to attach"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "Extra Trello fields as JSON, e.g. {\"urlSource\":\"https://...\"}"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Card ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Card"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := trello.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	listID, err := trello.RequiredString("list_id", inputs)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	name, err := trello.RequiredString("name", inputs)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	params := url.Values{}
	params.Set("idList", listID)
	params.Set("name", name)
	trello.SetIfPresent(params, inputs, "desc", "description")
	trello.SetIfPresent(params, inputs, "due", "due")
	trello.SetIfPresent(params, inputs, "pos", "position")
	trello.SetIfPresent(params, inputs, "idMembers", "member_ids")
	trello.SetIfPresent(params, inputs, "idLabels", "label_ids")
	if err := trello.MergeAdditionalFields(params, inputs); err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	obj, err := trello.WriteObject(auth, http.MethodPost, "/cards", params)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	return trello.ResourceResult(obj, fmt.Sprintf("Created card %q", name)), nil
}
