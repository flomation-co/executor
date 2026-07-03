package scheduling_calendly_invitation_create

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	calendly "flomation.app/automate/executor/actions/scheduling/calendly"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Calendly: Invite Member"
	Description  = "Invite a person to join your Calendly organisation by email."
	Website      = "https://www.flomation.co"
	Icon         = "calendly+plus"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Calendly Connection", Placeholder: "Calendly credential or Personal Access Token", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "person@example.com", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Invitation URI"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Invitation"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calendly.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	email, err := calendly.RequiredString("email", inputs)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}

	_, orgURI, err := calendly.CurrentUser(token)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}
	orgUUID := calendly.ExtractUUID(orgURI)

	body := map[string]interface{}{"email": email}
	resp, err := calendly.PostResource(token, "/organizations/"+url.PathEscape(orgUUID)+"/invitations", body)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}
	return calendly.ResourceResult(resp, fmt.Sprintf("Invited %s to the organisation", email)), nil
}
