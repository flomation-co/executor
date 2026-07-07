package trello_card_update

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	trello "flomation.app/automate/executor/actions/trello"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Update Card"
	Description  = "Change an existing Trello card — rename it, edit the description, set a due date, move it to another list or board, or update members and labels."
	Website      = "https://www.flomation.co"
	Icon         = "trello+pen"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Your Trello API key", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Trello API token", Required: true},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Card ID", Placeholder: "The ID of the card to update", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "A new name for the card"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "A new description for the card"},
	{Name: "due", Type: core.ConnectionTypeDateTime, Label: "Due Date", Placeholder: "A due date for the card"},
	{Name: "due_complete", Type: core.ConnectionTypeBoolean, Label: "Due Complete", Placeholder: "Mark the due date complete"},
	{Name: "list_id", Type: core.ConnectionTypeString, Label: "Move to List", Placeholder: "Move the card to this list ID"},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Move to Board", Placeholder: "Move the card to this board ID"},
	{Name: "position", Type: core.ConnectionTypeString, Label: "Position", Placeholder: "top, bottom, or a positive number"},
	{Name: "member_ids", Type: core.ConnectionTypeString, Label: "Member IDs", Placeholder: "Comma-separated member IDs (replaces the set)"},
	{Name: "label_ids", Type: core.ConnectionTypeString, Label: "Label IDs", Placeholder: "Comma-separated label IDs (replaces the set)"},
	{Name: "closed", Type: core.ConnectionTypeBoolean, Label: "Closed", Placeholder: "Archive the card, or reopen it"},
	{Name: "subscribed", Type: core.ConnectionTypeBoolean, Label: "Subscribed", Placeholder: "Subscribe to the card"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "Extra Trello fields as JSON"},
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
	id, err := trello.RequiredString("id", inputs)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	params := url.Values{}
	trello.SetIfPresent(params, inputs, "name", "name")
	trello.SetIfPresent(params, inputs, "desc", "description")
	trello.SetIfPresent(params, inputs, "due", "due")
	trello.SetBoolIfSet(params, inputs, "dueComplete", "due_complete")
	trello.SetIfPresent(params, inputs, "idList", "list_id")
	trello.SetIfPresent(params, inputs, "idBoard", "board_id")
	trello.SetIfPresent(params, inputs, "pos", "position")
	trello.SetIfPresent(params, inputs, "idMembers", "member_ids")
	trello.SetIfPresent(params, inputs, "idLabels", "label_ids")
	trello.SetBoolIfSet(params, inputs, "closed", "closed")
	trello.SetBoolIfSet(params, inputs, "subscribed", "subscribed")
	if err := trello.MergeAdditionalFields(params, inputs); err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	obj, err := trello.WriteObject(auth, http.MethodPut, "/cards/"+url.PathEscape(id), params)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	return trello.ResourceResult(obj, "Updated card "+id), nil
}
