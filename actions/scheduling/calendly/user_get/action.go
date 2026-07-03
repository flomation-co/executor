package scheduling_calendly_user_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	calendly "flomation.app/automate/executor/actions/scheduling/calendly"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Calendly: Get Current User"
	Description  = "Retrieve the Calendly user the connection belongs to, including their scheduling URL and organisation."
	Website      = "https://www.flomation.co"
	Icon         = "calendly+user"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Calendly Connection", Placeholder: "Calendly credential or Personal Access Token", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "User URI"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email"},
	{Name: "scheduling_url", Type: core.ConnectionTypeString, Label: "Scheduling URL"},
	{Name: "organization", Type: core.ConnectionTypeString, Label: "Organization URI"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "User"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calendly.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	resp, err := calendly.GetResource(token, "/users/me", nil)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}
	resource, _ := resp["resource"].(map[string]interface{})
	if resource == nil {
		return calendly.ErrorResult("Calendly returned no user resource"), nil
	}

	name, _ := resource["name"].(string)
	out := calendly.ResourceResult(resp, fmt.Sprintf("Retrieved Calendly user %s", name))
	out["email"], _ = resource["email"].(string)
	out["scheduling_url"], _ = resource["scheduling_url"].(string)
	out["organization"], _ = resource["current_organization"].(string)
	return out, nil
}
