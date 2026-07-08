package helpdesk_intercom_conversation_assign

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Assign Conversation"
	Description  = "Assign an Intercom conversation to a teammate or a team, or unassign it."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+user-plus"
	Date         = "08/07/2026"
	Type         = core.ActionTypeAction
)

var assignAdmin = &core.VisibleWhen{Field: "assignee_type", Values: []string{"admin", ""}}
var assignTeam = &core.VisibleWhen{Field: "assignee_type", Values: []string{"team"}}

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
	{Name: "conversation_id", Type: core.ConnectionTypeString, Label: "Conversation ID", Placeholder: "The ID of the conversation to assign", Required: true},
	{Name: "admin_id", Type: core.ConnectionTypeString, Label: "Acting Admin", Placeholder: "The teammate performing the assignment", Required: true},
	{
		Name:  "assignee_type",
		Type:  core.ConnectionTypeString,
		Label: "Assign To",
		Options: []core.ConnectionOption{
			{Name: "Admin", Value: "admin"},
			{Name: "Team", Value: "team"},
			{Name: "Unassign", Value: "unassign"},
		},
	},
	{Name: "assignee_admin_id", Type: core.ConnectionTypeString, Label: "Assignee (Admin)", Placeholder: "The teammate to hand the conversation to", Visible: assignAdmin},
	{Name: "assignee_team_id", Type: core.ConnectionTypeString, Label: "Assignee (Team)", Placeholder: "The team to hand the conversation to", Visible: assignTeam},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Conversation ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Conversation"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := intercom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, err := intercom.RequiredString("conversation_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	adminID, err := intercom.RequiredString("admin_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	assigneeType := intercom.OptionalString("assignee_type", inputs)
	if assigneeType == "" {
		assigneeType = "admin"
	}

	var partType, assigneeID, summary string
	switch assigneeType {
	case "admin":
		assigneeID = intercom.OptionalString("assignee_admin_id", inputs)
		if assigneeID == "" {
			return intercom.ErrorResult("pick the Assignee (Admin) to hand the conversation to, or switch Assign To to Team or Unassign"), nil
		}
		partType = "admin"
		summary = "Assigned conversation " + id + " to admin " + assigneeID
	case "team":
		assigneeID = intercom.OptionalString("assignee_team_id", inputs)
		if assigneeID == "" {
			return intercom.ErrorResult("pick the Assignee (Team) to hand the conversation to"), nil
		}
		partType = "team"
		summary = "Assigned conversation " + id + " to team " + assigneeID
	case "unassign":
		// Unassignment is an admin-typed assignment to the null assignee "0".
		partType = "admin"
		assigneeID = "0"
		summary = "Unassigned conversation " + id
	default:
		return intercom.ErrorResult("Assign To must be Admin, Team, or Unassign"), nil
	}
	body := map[string]interface{}{
		"message_type": "assignment",
		"type":         partType,
		"admin_id":     adminID,
		"assignee_id":  assigneeID,
	}

	obj, err := intercom.WriteObject(auth, http.MethodPost, "/conversations/"+url.PathEscape(id)+"/parts", body, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.ResourceResult(obj, summary), nil
}
