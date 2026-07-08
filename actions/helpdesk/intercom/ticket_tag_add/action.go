package helpdesk_intercom_ticket_tag_add

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Add Tag to Ticket"
	Description  = "Add an existing tag to an Intercom ticket so it's easy to group, filter, and report on."
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
	{Name: "ticket_id", Type: core.ConnectionTypeString, Label: "Ticket ID", Placeholder: "The ticket to tag — from a Create Ticket step or a search", Required: true},
	{Name: "tag_id", Type: core.ConnectionTypeString, Label: "Tag", Placeholder: "The tag to add — it must already exist in Intercom", Required: true},
	{Name: "admin_id", Type: core.ConnectionTypeString, Label: "Admin", Placeholder: "The teammate applying the tag (for attribution)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Ticket ID"},
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
	ticketID, err := intercom.RequiredString("ticket_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	tagID, err := intercom.RequiredString("tag_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	adminID, err := intercom.RequiredString("admin_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"id": tagID, "admin_id": adminID}
	obj, err := intercom.WriteObject(auth, http.MethodPost, "/tickets/"+url.PathEscape(ticketID)+"/tags", body, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	summary := fmt.Sprintf("Added tag %s to ticket %s", tagID, ticketID)
	if name, ok := obj["name"].(string); ok && name != "" {
		summary = fmt.Sprintf("Added tag %q to ticket %s", name, ticketID)
	}
	return intercom.SuccessResult(ticketID, obj, summary), nil
}
