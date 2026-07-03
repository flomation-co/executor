package scheduling_calendly_membership_delete

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	calendly "flomation.app/automate/executor/actions/scheduling/calendly"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Calendly: Remove Member"
	Description  = "Remove a member from your Calendly organisation."
	Website      = "https://www.flomation.co"
	Icon         = "calendly+trash"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Calendly Connection", Placeholder: "Calendly credential or Personal Access Token", Required: true},
	{Name: "membership", Type: core.ConnectionTypeString, Label: "Membership", Placeholder: "Organisation membership ID or URI", Required: true},
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
	id, err := calendly.RequiredString("membership", inputs)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}

	uuid := calendly.ExtractUUID(id)
	if err := calendly.DeleteResource(token, "/organization_memberships/"+url.PathEscape(uuid)); err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Removed organisation member %s", uuid),
		"success":     true,
		"error":       "",
	}, nil
}
