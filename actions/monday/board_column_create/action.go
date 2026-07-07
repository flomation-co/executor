package monday_board_column_create

import (
	core "flomation.app/automate/executor"
	monday "flomation.app/automate/executor/actions/monday"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Create Column"
	Description  = "Add a column to a Monday.com board — choose its title and type (text, status, date, numbers, people, etc.)."
	Website      = "https://www.flomation.co"
	Icon         = "monday+plus"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Monday.com API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board to add the column to", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Placeholder: "The column title", Required: true},
	{Name: "column_type", Type: core.ConnectionTypeString, Label: "Type", Placeholder: "The column type", Required: true, Options: []core.ConnectionOption{
		{Name: "Text", Value: "text"},
		{Name: "Long Text", Value: "long_text"},
		{Name: "Numbers", Value: "numbers"},
		{Name: "Status", Value: "status"},
		{Name: "Dropdown", Value: "dropdown"},
		{Name: "Date", Value: "date"},
		{Name: "People", Value: "people"},
		{Name: "Timeline", Value: "timeline"},
		{Name: "Tags", Value: "tags"},
		{Name: "Checkbox", Value: "checkbox"},
		{Name: "Link", Value: "link"},
		{Name: "Email", Value: "email"},
		{Name: "Phone", Value: "phone"},
		{Name: "Rating", Value: "rating"},
	}},
	{Name: "defaults", Type: core.ConnectionTypeObject, Label: "Defaults", Placeholder: "Optional column defaults as JSON (e.g. status labels)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Column ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Column"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := monday.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	boardID, err := monday.RequiredString("board_id", inputs)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	title, err := monday.RequiredString("title", inputs)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	columnType, err := monday.RequiredString("column_type", inputs)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	defaults, err := monday.ValidateJSON("defaults", inputs)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	vars := map[string]interface{}{"boardId": boardID, "title": title, "columnType": columnType}
	if defaults != "" {
		vars["defaults"] = defaults
	}
	data, err := monday.GraphQL(auth, `mutation ($boardId: ID!, $title: String!, $columnType: ColumnType!, $defaults: JSON) {
		create_column (board_id: $boardId, title: $title, column_type: $columnType, defaults: $defaults) {
			id
			title
			type
		}
	}`, vars)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	return monday.ResourceResult(monday.ObjectField(data, "create_column"), "Created column "+title), nil
}
