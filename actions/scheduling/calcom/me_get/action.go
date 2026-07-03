package scheduling_calcom_me_get

import (
	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Get My Profile"
	Description  = "Retrieve the Cal.com profile of the connected account (id, username, timezone, default schedule)."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+magnifying-glass"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "User ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Profile"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calcom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	resp, err := calcom.GetResource(token, "/me", calcom.VersionNone, nil)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	summary := "Retrieved Cal.com profile"
	if u, ok := resp["username"].(string); ok && u != "" {
		summary = "Retrieved Cal.com profile for " + u
	}
	return calcom.ResourceResult(resp, summary), nil
}
