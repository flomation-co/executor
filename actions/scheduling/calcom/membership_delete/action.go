package scheduling_calcom_membership_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Remove Team Member"
	Description  = "Remove a member from a Cal.com team."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+trash"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "team_id", Type: core.ConnectionTypeInteger, Label: "Team ID", Required: true},
	{Name: "membership_id", Type: core.ConnectionTypeInteger, Label: "Membership ID", Required: true},
}

var Outputs = [...]core.Connection{
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
	membershipID, err := calcom.RequiredInt("membership_id", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}

	path := fmt.Sprintf("/teams/%d/memberships/%d", teamID, membershipID)
	if err := calcom.DeleteResource(token, path, calcom.VersionNone); err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Removed membership %d from team %d", membershipID, teamID),
		"success":     true,
		"error":       "",
	}, nil
}
