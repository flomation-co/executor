package helpdesk_intercom_tag_create_update

import (
	"net/http"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Create or Update Tag"
	Description  = "Create a tag in Intercom, or rename an existing one by also providing its Tag ID. Creating a name that already exists simply returns the existing tag."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+plus"
	Date         = "08/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Intercom access token (Developer Hub → Authentication)", Required: true},
	{
		Name:  "region",
		Type:  core.ConnectionTypeString,
		Label: "Region",
		Options: []core.ConnectionOption{
			{Name: "US (default)", Value: "us"},
			{Name: "Europe", Value: "eu"},
			{Name: "Australia", Value: "au"},
		},
	},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Tag Name", Placeholder: "VIP — created if it doesn't exist yet", Required: true},
	{Name: "tag_id", Type: core.ConnectionTypeString, Label: "Tag ID (rename)", Placeholder: "Leave empty to create; provide an existing tag's ID to rename it"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Tag ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Tag"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := intercom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	name, err := intercom.RequiredString("name", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	// POST /tags is Intercom's create-or-rename endpoint: {name} creates (or
	// returns the existing tag of that name); {name, id} renames tag id.
	body := map[string]interface{}{"name": name}
	tagID := intercom.OptionalString("tag_id", inputs)
	if tagID != "" {
		body["id"] = tagID
	}

	obj, err := intercom.WriteObject(auth, http.MethodPost, "/tags", body, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	summary := "Created tag " + name
	if tagID != "" {
		summary = "Renamed tag " + tagID + " to " + name
	}
	return intercom.ResourceResult(obj, summary), nil
}
