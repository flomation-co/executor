package helpdesk_intercom_contact_tag_add

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Add Tag to Contact"
	Description  = "Apply a tag to a contact — handy for segmenting people. Pick from your workspace's existing tags (use Create/Update Tag to make a new one)."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+hashtag"
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
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "The contact's Intercom ID", Required: true},
	{Name: "tag_id", Type: core.ConnectionTypeString, Label: "Tag", Placeholder: "The tag to apply", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Contact ID"},
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

	contactID, err := intercom.RequiredString("contact_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	tagID, err := intercom.RequiredString("tag_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"id": tagID}
	// Intercom echoes the applied tag object ({"type":"tag","id","name"}).
	obj, err := intercom.WriteObject(auth, http.MethodPost, "/contacts/"+url.PathEscape(contactID)+"/tags", body, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	label := tagID
	if name, _ := obj["name"].(string); name != "" {
		label = name
	}
	return intercom.SuccessResult(contactID, obj, "Added tag "+label+" to contact "+contactID), nil
}
