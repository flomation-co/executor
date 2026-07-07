package trello_board_create

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
	Name         = "Create Board"
	Description  = "Create a new Trello board. Give it a name and an optional description; use Additional Fields for board preferences (visibility, background, etc.)."
	Website      = "https://www.flomation.co"
	Icon         = "trello+plus"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Your Trello API key", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Trello API token", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "The name of the new board", Required: true},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "An optional description for the board"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "Extra Trello fields as JSON, e.g. {\"prefs_permissionLevel\":\"org\",\"idOrganization\":\"...\"}"},
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
	name, err := trello.RequiredString("name", inputs)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	params := url.Values{}
	params.Set("name", name)
	trello.SetIfPresent(params, inputs, "desc", "description")
	if err := trello.MergeAdditionalFields(params, inputs); err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	obj, err := trello.WriteObject(auth, http.MethodPost, "/boards", params)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	return trello.ResourceResult(obj, fmt.Sprintf("Created board %q", name)), nil
}
