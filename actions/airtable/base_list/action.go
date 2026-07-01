package airtable_base_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	airtable "flomation.app/automate/executor/actions/airtable"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Base: List"
	Description  = "List the Airtable bases the token can access, with their IDs and permission levels. Optionally filter by permission level. Requires the schema.bases:read scope."
	Website      = "https://www.flomation.co"
	Icon         = "airtable+list"
	Date         = "01/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Airtable Personal Access Token", Placeholder: "pat...", Required: true},
	{Name: "permission_level", Type: core.ConnectionTypeString, Label: "Permission Level", Placeholder: "Comma-separated filter: read, comment, edit, create (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "bases", Type: core.ConnectionTypeObject, Label: "Bases"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := airtable.GetAccessToken(inputs)
	if err != nil {
		return nil, err
	}

	var all []interface{}
	offset := ""
	pages := 0
	for {
		bases, next, _, err := airtable.ListBasesPage(token, offset)
		if err != nil {
			return airtable.ErrorResult(err.Error()), nil
		}
		all = append(all, bases...)
		offset = next
		pages++
		if offset == "" || pages >= airtable.MaxAllPages {
			break
		}
	}

	// Optional client-side filter by permission level (Airtable's /meta/bases
	// endpoint has no server-side filter; each base carries permissionLevel).
	if levels := airtable.CSVToList(airtable.OptionalString("permission_level", inputs)); len(levels) > 0 {
		allowed := make(map[string]bool, len(levels))
		for _, l := range levels {
			allowed[l] = true
		}
		filtered := make([]interface{}, 0, len(all))
		for _, b := range all {
			if bm, ok := b.(map[string]interface{}); ok {
				if lvl, _ := bm["permissionLevel"].(string); allowed[lvl] {
					filtered = append(filtered, bm)
				}
			}
		}
		all = filtered
	}

	return map[string]interface{}{
		"bases":       all,
		"count":       len(all),
		"tool_result": fmt.Sprintf("Found %d base(s)", len(all)),
		"success":     true,
		"error":       "",
	}, nil
}
