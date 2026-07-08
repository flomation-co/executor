package helpdesk_intercom_admin_away_set

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Set Admin Away Mode"
	Description  = "Turn an admin's away mode on or off, and choose whether their new conversations are reassigned to the rest of the team while they're away."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+clock"
	Date         = "08/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Intercom access token (Developer Hub → Authentication)", Required: true},
	{
		Name:  "region",
		Type:  core.ConnectionTypeString,
		Label: "Region",
		Options: []core.ConnectionOption{
			{Name: "US (default)", Value: "us"},
			{Name: "Europe", Value: "eu"},
			{Name: "Australia", Value: "au"},
		},
	},
	{Name: "admin_id", Type: core.ConnectionTypeString, Label: "Admin", Placeholder: "The teammate whose away mode to change", Required: true},
	{Name: "away_mode_enabled", Type: core.ConnectionTypeBoolean, Label: "Away Mode On", Placeholder: "Tick to mark the admin as away; leave unticked to bring them back"},
	{Name: "away_mode_reassign", Type: core.ConnectionTypeBoolean, Label: "Reassign New Conversations", Placeholder: "While away, route their new conversations to the rest of the team"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Admin ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Admin"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := intercom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	adminID, err := intercom.RequiredString("admin_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	// Intercom requires BOTH booleans on every call, so an untouched checkbox
	// is sent as false rather than omitted.
	enabled, _ := intercom.OptionalBoolSet("away_mode_enabled", inputs)
	reassign, _ := intercom.OptionalBoolSet("away_mode_reassign", inputs)
	body := map[string]interface{}{
		"away_mode_enabled":  enabled,
		"away_mode_reassign": reassign,
	}

	obj, err := intercom.WriteObject(auth, http.MethodPut, "/admins/"+url.PathEscape(adminID)+"/away", body, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	label, _ := obj["name"].(string)
	if label == "" {
		label = adminID
	}
	state := "off"
	if enabled {
		state = "on"
	}
	return intercom.ResourceResult(obj, fmt.Sprintf("Away mode turned %s for %s", state, label)), nil
}
