package trello_list_archive

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	trello "flomation.app/automate/executor/actions/trello"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Archive/Unarchive List"
	Description  = "Archive (close) a Trello list, or reopen it, with the Archive toggle."
	Website      = "https://www.flomation.co"
	Icon         = "trello+box-archive"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Your Trello API key", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Trello API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board that owns the list (used to load the list picker)"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "List", Placeholder: "The list to archive or unarchive", Required: true},
	{Name: "archive", Type: core.ConnectionTypeBoolean, Label: "Archive", Placeholder: "On = archive the list; off = reopen it (defaults to archive)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "List ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "List"},
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
	// Trello's /closed endpoint requires a value, so we must always send one.
	// When the operator never touched the toggle, default to archiving — that is
	// the primary purpose of this action; defaulting to false would silently
	// REOPEN the target list on a freshly-dropped node.
	archive, set := trello.OptionalBoolSet("archive", inputs)
	if !set {
		archive = true
	}
	params := url.Values{}
	params.Set("value", boolStr(archive))
	obj, err := trello.WriteObject(auth, http.MethodPut, "/lists/"+url.PathEscape(id)+"/closed", params)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	verb := "Unarchived"
	if archive {
		verb = "Archived"
	}
	return trello.ResourceResult(obj, verb+" list "+id), nil
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
