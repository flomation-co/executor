package scheduling_calcom_membership_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Update Team Member"
	Description  = "Change a team member's role or accepted status in a Cal.com team."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+pencil"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "team_id", Type: core.ConnectionTypeInteger, Label: "Team ID", Required: true},
	{Name: "membership_id", Type: core.ConnectionTypeInteger, Label: "Membership ID", Required: true},
	{Name: "role", Type: core.ConnectionTypeString, Label: "Role", Options: []core.ConnectionOption{
		{Name: "(unchanged)", Value: ""},
		{Name: "Member", Value: "MEMBER"},
		{Name: "Admin", Value: "ADMIN"},
		{Name: "Owner", Value: "OWNER"},
	}},
	{Name: "accepted", Type: core.ConnectionTypeBoolean, Label: "Accepted"},
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
	membershipID, err := calcom.RequiredInt("membership_id", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{}
	calcom.SetIfString(body, inputs, "role", "role")
	calcom.SetIfBoolPresent(body, inputs, "accepted", "accepted")
	if len(body) == 0 {
		return calcom.ErrorResult("no fields to update: supply a role or accepted status"), nil
	}

	path := fmt.Sprintf("/teams/%d/memberships/%d", teamID, membershipID)
	resp, err := calcom.PatchResource(token, path, calcom.VersionNone, body)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return calcom.ResourceResult(resp, fmt.Sprintf("Updated membership %d in team %d", membershipID, teamID)), nil
}
