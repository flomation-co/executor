package helpdesk_zendesk_user_update

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	zendesk "flomation.app/automate/executor/actions/helpdesk/zendesk"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Zendesk: Update User"
	Description  = "Update a Zendesk user. Change profile fields, custom user fields, or any other field via Additional Fields."
	Website      = "https://www.flomation.co"
	Icon         = "zendesk+pencil"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "subdomain", Type: core.ConnectionTypeString, Label: "Subdomain", Placeholder: "mycompany (from mycompany.zendesk.com)", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Agent Email", Placeholder: "you@company.com (paired with the API token)"},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Zendesk API token"},
	{Name: "oauth_token", Type: core.ConnectionTypeSecret, Label: "OAuth Access Token", Placeholder: "Optional — a bearer token used instead of the email + API token"},
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User ID", Placeholder: "35436", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "The user's full name"},
	// Named user_email (not email) so it never collides with the auth "email"
	// input above — otherwise the agent's auth email would leak into the user.
	{Name: "user_email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "The user's primary email address"},
	{
		Name:  "role",
		Type:  core.ConnectionTypeString,
		Label: "Role",
		Options: []core.ConnectionOption{
			{Name: "End User", Value: "end-user"},
			{Name: "Agent", Value: "agent"},
			{Name: "Admin", Value: "admin"},
		},
	},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Phone", Placeholder: "+1 555 123 4567"},
	{Name: "alias", Type: core.ConnectionTypeString, Label: "Alias", Placeholder: "An agent alias displayed to end users"},
	{Name: "organization_id", Type: core.ConnectionTypeString, Label: "Organization", Placeholder: "The organization this user belongs to (ID)"},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "External ID", Placeholder: "An ID linking this user to a record in another system"},
	{Name: "details", Type: core.ConnectionTypeText, Label: "Details", Placeholder: "Details about the user, such as an address"},
	{Name: "notes", Type: core.ConnectionTypeText, Label: "Notes", Placeholder: "Notes about the user"},
	{Name: "time_zone", Type: core.ConnectionTypeString, Label: "Time Zone", Placeholder: "e.g. Pacific Time (US & Canada)"},
	{Name: "locale", Type: core.ConnectionTypeString, Label: "Locale", Placeholder: "e.g. en-US"},
	{Name: "signature", Type: core.ConnectionTypeText, Label: "Signature", Placeholder: "The agent's signature (agents and admins only)"},
	{Name: "custom_role_id", Type: core.ConnectionTypeInteger, Label: "Custom Role ID", Placeholder: "The ID of a custom agent role"},
	{
		Name:  "ticket_restriction",
		Type:  core.ConnectionTypeString,
		Label: "Ticket Restriction",
		Options: []core.ConnectionOption{
			{Name: "Organization", Value: "organization"},
			{Name: "Groups", Value: "groups"},
			{Name: "Assigned", Value: "assigned"},
			{Name: "Requested", Value: "requested"},
		},
	},
	{Name: "moderator", Type: core.ConnectionTypeBoolean, Label: "Moderator", Placeholder: "Whether the user has forum moderation capabilities"},
	{Name: "only_private_comments", Type: core.ConnectionTypeBoolean, Label: "Only Private Comments", Placeholder: "Whether the user can only create private comments"},
	{Name: "restricted_agent", Type: core.ConnectionTypeBoolean, Label: "Restricted Agent", Placeholder: "Whether the agent has restrictions"},
	{Name: "report_csv", Type: core.ConnectionTypeBoolean, Label: "Report CSV", Placeholder: "Whether the user can access the CSV report"},
	{Name: "suspended", Type: core.ConnectionTypeBoolean, Label: "Suspended", Placeholder: "Whether the user is suspended"},
	{Name: "verified", Type: core.ConnectionTypeBoolean, Label: "Verified", Placeholder: "Whether the user's identities are verified"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Tags", Placeholder: "vip, wholesale (comma-separated) — replaces the user's tags"},
	{Name: "user_fields", Type: core.ConnectionTypeObject, Label: "User Fields (JSON)", Placeholder: `{"support_plan":"gold"}`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"shared_phone_number":true}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "User ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "User"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	subdomain, auth, err := zendesk.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, err := zendesk.RequiredString("user_id", inputs)
	if err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}

	user := map[string]interface{}{}
	zendesk.SetIfPresent(user, inputs, "name", "name")
	zendesk.SetIfPresent(user, inputs, "email", "user_email")
	zendesk.SetIfPresent(user, inputs, "role", "role")
	zendesk.SetIfPresent(user, inputs, "phone", "phone")
	zendesk.SetIfPresent(user, inputs, "alias", "alias")
	zendesk.SetNumericIDIfPresent(user, inputs, "organization_id", "organization_id")
	zendesk.SetIfPresent(user, inputs, "external_id", "external_id")
	zendesk.SetIfPresent(user, inputs, "details", "details")
	zendesk.SetIfPresent(user, inputs, "notes", "notes")
	zendesk.SetIfPresent(user, inputs, "time_zone", "time_zone")
	zendesk.SetIfPresent(user, inputs, "locale", "locale")
	zendesk.SetIfPresent(user, inputs, "signature", "signature")
	zendesk.SetIntIfSet(user, inputs, "custom_role_id", "custom_role_id")
	zendesk.SetIfPresent(user, inputs, "ticket_restriction", "ticket_restriction")
	zendesk.SetBoolIfSet(user, inputs, "moderator", "moderator")
	zendesk.SetBoolIfSet(user, inputs, "only_private_comments", "only_private_comments")
	zendesk.SetBoolIfSet(user, inputs, "restricted_agent", "restricted_agent")
	zendesk.SetBoolIfSet(user, inputs, "report_csv", "report_csv")
	zendesk.SetBoolIfSet(user, inputs, "suspended", "suspended")
	zendesk.SetBoolIfSet(user, inputs, "verified", "verified")
	zendesk.SetCSVIfPresent(user, inputs, "tags", "tags")
	if err := zendesk.SetJSONIfPresent(user, inputs, "user_fields", "user_fields"); err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}
	if err := zendesk.MergeAdditionalFields(user, inputs); err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}

	resp, err := zendesk.UpdateResource(subdomain, auth, "/users/"+url.PathEscape(id)+".json", "user", user)
	if err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}
	out := zendesk.ResourceResult(resp, "user", fmt.Sprintf("Updated user %s", id))
	return out, nil
}
