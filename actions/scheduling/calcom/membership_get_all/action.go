package scheduling_calcom_membership_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Get Many Team Members"
	Description  = "List the members (memberships) of a Cal.com team."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+list"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "team_id", Type: core.ConnectionTypeInteger, Label: "Team ID", Required: true},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Results per page (1-250, default 100)"},
	{Name: "skip", Type: core.ConnectionTypeInteger, Label: "Skip", Placeholder: "Offset to resume from a previous run's next_skip"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow pagination and return every member"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Memberships"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "next_skip", Type: core.ConnectionTypeInteger, Label: "Next Skip"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calcom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	teamID, ok := calcom.OptionalInt("team_id", inputs)
	if !ok {
		return calcom.ErrorResult("team_id is required"), nil
	}

	limit, set := calcom.OptionalInt("limit", inputs)
	skip, _ := calcom.OptionalInt("skip", inputs)
	returnAll := calcom.OptionalBool("return_all", inputs)

	path := fmt.Sprintf("/teams/%d/memberships", teamID)
	items, next, _, err := calcom.ListResources(token, path, calcom.VersionNone, nil, calcom.ClampLimit(limit, set), skip, returnAll)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return calcom.ListResult(items, next, fmt.Sprintf("Retrieved %d team members", len(items))), nil
}
