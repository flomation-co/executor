package scheduling_calendly_invitation_delete

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	calendly "flomation.app/automate/executor/actions/scheduling/calendly"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Calendly: Revoke Invitation"
	Description  = "Revoke a pending invitation to join your Calendly organisation."
	Website      = "https://www.flomation.co"
	Icon         = "calendly+trash"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Calendly Connection", Placeholder: "Calendly credential or Personal Access Token", Required: true},
	{Name: "invitation", Type: core.ConnectionTypeString, Label: "Invitation", Placeholder: "Invitation ID or URI", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calendly.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, err := calendly.RequiredString("invitation", inputs)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}

	_, orgURI, err := calendly.CurrentUser(token)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}
	orgUUID := calendly.ExtractUUID(orgURI)

	uuid := calendly.ExtractUUID(id)
	if err := calendly.DeleteResource(token, "/organizations/"+url.PathEscape(orgUUID)+"/invitations/"+url.PathEscape(uuid)); err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Revoked invitation %s", uuid),
		"success":     true,
		"error":       "",
	}, nil
}
