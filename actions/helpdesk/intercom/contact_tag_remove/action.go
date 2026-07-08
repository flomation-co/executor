package helpdesk_intercom_contact_tag_remove

import (
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Remove Tag from Contact"
	Description  = "Take a tag off a contact. The tag itself isn't deleted — it just no longer applies to this person."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+minus"
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
	{Name: "tag_id", Type: core.ConnectionTypeString, Label: "Tag", Placeholder: "The tag to remove", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Contact ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
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

	// Unlike conversation/ticket untag, contact untag takes no admin_id body —
	// a plain DELETE on the tag path is the whole call.
	if err := intercom.DeleteResource(auth, "/contacts/"+url.PathEscape(contactID)+"/tags/"+url.PathEscape(tagID)); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	result := map[string]interface{}{"contact_id": contactID, "tag_id": tagID}
	return intercom.SuccessResult(contactID, result, "Removed tag "+tagID+" from contact "+contactID), nil
}
