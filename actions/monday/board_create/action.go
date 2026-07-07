package monday_board_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	monday "flomation.app/automate/executor/actions/monday"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Create Board"
	Description  = "Create a new Monday.com board. Choose its name and kind (public, private, or shareable), optionally in a workspace or from a template."
	Website      = "https://www.flomation.co"
	Icon         = "monday+plus"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Monday.com API token", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "The name of the board", Required: true},
	{Name: "kind", Type: core.ConnectionTypeString, Label: "Kind", Placeholder: "Board visibility", Required: true, Options: []core.ConnectionOption{
		{Name: "Public", Value: "public"},
		{Name: "Private", Value: "private"},
		{Name: "Shareable", Value: "share"},
	}},
	{Name: "workspace_id", Type: core.ConnectionTypeString, Label: "Workspace", Placeholder: "Optionally create the board in this workspace"},
	{Name: "template_id", Type: core.ConnectionTypeString, Label: "Template ID", Placeholder: "Optionally create from this board template"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Board ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Board"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := monday.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	name, err := monday.RequiredString("name", inputs)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	kind, err := monday.RequiredString("kind", inputs)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	vars := map[string]interface{}{"name": name, "kind": kind}
	if v := monday.OptionalString("workspace_id", inputs); v != "" {
		vars["workspaceId"] = v
	}
	if v := monday.OptionalString("template_id", inputs); v != "" {
		vars["templateId"] = v
	}
	data, err := monday.GraphQL(auth, `mutation ($name: String!, $kind: BoardKind!, $workspaceId: ID, $templateId: ID) {
		create_board (board_name: $name, board_kind: $kind, workspace_id: $workspaceId, template_id: $templateId) {
			id
			name
		}
	}`, vars)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	return monday.ResourceResult(monday.ObjectField(data, "create_board"), fmt.Sprintf("Created board %q", name)), nil
}
