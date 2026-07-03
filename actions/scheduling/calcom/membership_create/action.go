package scheduling_calcom_membership_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Add Team Member"
	Description  = "Add a user to a Cal.com team as a member, admin or owner."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+user-plus"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "team_id", Type: core.ConnectionTypeInteger, Label: "Team ID", Required: true},
	{Name: "user_id", Type: core.ConnectionTypeInteger, Label: "User ID", Placeholder: "The Cal.com user to add", Required: true},
	{Name: "role", Type: core.ConnectionTypeString, Label: "Role", Options: []core.ConnectionOption{
		{Name: "Member", Value: "MEMBER"},
		{Name: "Admin", Value: "ADMIN"},
		{Name: "Owner", Value: "OWNER"},
	}},
	{Name: "accepted", Type: core.ConnectionTypeBoolean, Label: "Auto-Accept", Placeholder: "Add without requiring the user to accept an invite"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Membership ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Membership"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calcom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	teamID, err := calcom.RequiredInt("team_id", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	userID, err := calcom.RequiredInt("user_id", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"userId": userID}
	calcom.SetIfString(body, inputs, "role", "role")
	calcom.SetIfBoolPresent(body, inputs, "accepted", "accepted")

	path := fmt.Sprintf("/teams/%d/memberships", teamID)
	resp, err := calcom.PostResource(token, path, calcom.VersionNone, body)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return calcom.ResourceResult(resp, fmt.Sprintf("Added user %d to team %d", userID, teamID)), nil
}
