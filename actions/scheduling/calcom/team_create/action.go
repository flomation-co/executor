package scheduling_calcom_team_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Create Team"
	Description  = "Create a new Cal.com team."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+plus"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "e.g. Sales Team", Required: true},
	{Name: "slug", Type: core.ConnectionTypeString, Label: "Slug", Placeholder: "e.g. sales-team (auto-generated if blank)"},
	{Name: "bio", Type: core.ConnectionTypeText, Label: "Bio", Placeholder: "Shown on the team page (optional)"},
	{Name: "time_zone", Type: core.ConnectionTypeString, Label: "Time Zone", Placeholder: "Europe/London (optional)"},
	{Name: "hide_branding", Type: core.ConnectionTypeBoolean, Label: "Hide Cal.com Branding"},
	{Name: "extra_json", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON, advanced)", Placeholder: `{"brandColor":"#292929"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Team ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Team"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calcom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	name, err := calcom.RequiredString("name", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"name": name}
	calcom.SetIfString(body, inputs, "slug", "slug")
	calcom.SetIfString(body, inputs, "bio", "bio")
	calcom.SetIfString(body, inputs, "timeZone", "time_zone")
	calcom.SetIfBoolPresent(body, inputs, "hideBranding", "hide_branding")
	if extra, err := calcom.ParseJSONObject("extra_json", inputs); err != nil {
		return calcom.ErrorResult(err.Error()), nil
	} else {
		for k, v := range extra {
			// Advanced JSON augments the body but never clobbers a validated field.
			if _, exists := body[k]; !exists {
				body[k] = v
			}
		}
	}

	resp, err := calcom.PostResource(token, "/teams", calcom.VersionNone, body)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return calcom.ResourceResult(resp, fmt.Sprintf("Created team %q", name)), nil
}
