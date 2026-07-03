package helpdesk_zendesk_user_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	zendesk "flomation.app/automate/executor/actions/helpdesk/zendesk"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Zendesk: Create User"
	Description  = "Create a user in Zendesk. Set common profile fields directly, add custom user fields as JSON, or add any other field via Additional Fields."
	Website      = "https://www.flomation.co"
	Icon         = "zendesk+plus"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "subdomain", Type: core.ConnectionTypeString, Label: "Subdomain", Placeholder: "mycompany (from mycompany.zendesk.com)", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Agent Email", Placeholder: "you@company.com (paired with the API token)"},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Zendesk API token"},
	{Name: "oauth_token", Type: core.ConnectionTypeSecret, Label: "OAuth Access Token", Placeholder: "Optional — a bearer token used instead of the email + API token"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "The user's full name", Required: true},
	// Named user_email (not email) so it never collides with the auth "email"
	// input above — otherwise the agent's auth email would leak into the new user.
	{Name: "user_email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "name@email.com"},
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
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Phone"},
	{Name: "alias", Type: core.ConnectionTypeString, Label: "Alias", Placeholder: "An alias displayed to end users"},
	{Name: "organization_id", Type: core.ConnectionTypeString, Label: "Organization", Placeholder: "Default organization (ID)"},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "External ID", Placeholder: "A unique identifier from another system"},
	{Name: "details", Type: core.ConnectionTypeString, Label: "Details", Placeholder: "Details such as an address"},
	{Name: "notes", Type: core.ConnectionTypeText, Label: "Notes"},
	{Name: "time_zone", Type: core.ConnectionTypeString, Label: "Timezone", Placeholder: "e.g. Pacific Time (US & Canada)"},
	{Name: "locale", Type: core.ConnectionTypeString, Label: "Locale", Placeholder: "e.g. en-US"},
	{Name: "signature", Type: core.ConnectionTypeString, Label: "Signature", Placeholder: "Agent/admin signature"},
	{Name: "custom_role_id", Type: core.ConnectionTypeInteger, Label: "Custom Role ID"},
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
	{Name: "moderator", Type: core.ConnectionTypeBoolean, Label: "Moderator"},
	{Name: "only_private_comments", Type: core.ConnectionTypeBoolean, Label: "Only Private Comments"},
	{Name: "restricted_agent", Type: core.ConnectionTypeBoolean, Label: "Restricted Agent"},
	{Name: "report_csv", Type: core.ConnectionTypeBoolean, Label: "Report CSV"},
	{Name: "suspended", Type: core.ConnectionTypeBoolean, Label: "Suspended"},
	{Name: "verified", Type: core.ConnectionTypeBoolean, Label: "Verified"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Tags", Placeholder: "vip, priority (comma-separated)"},
	{Name: "user_fields", Type: core.ConnectionTypeObject, Label: "User Fields (JSON)", Placeholder: `{"support_plan":"gold"}`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)"},
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

	name, err := zendesk.RequiredString("name", inputs)
	if err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"name": name}
	zendesk.SetIfPresent(body, inputs, "email", "user_email")
	zendesk.SetIfPresent(body, inputs, "role", "role")
	zendesk.SetIfPresent(body, inputs, "phone", "phone")
	zendesk.SetIfPresent(body, inputs, "alias", "alias")
	zendesk.SetIfPresent(body, inputs, "external_id", "external_id")
	zendesk.SetIfPresent(body, inputs, "details", "details")
	zendesk.SetIfPresent(body, inputs, "notes", "notes")
	zendesk.SetIfPresent(body, inputs, "time_zone", "time_zone")
	zendesk.SetIfPresent(body, inputs, "locale", "locale")
	zendesk.SetIfPresent(body, inputs, "signature", "signature")
	zendesk.SetNumericIDIfPresent(body, inputs, "organization_id", "organization_id")
	zendesk.SetIntIfSet(body, inputs, "custom_role_id", "custom_role_id")
	zendesk.SetIfPresent(body, inputs, "ticket_restriction", "ticket_restriction")
	zendesk.SetBoolIfSet(body, inputs, "moderator", "moderator")
	zendesk.SetBoolIfSet(body, inputs, "only_private_comments", "only_private_comments")
	zendesk.SetBoolIfSet(body, inputs, "restricted_agent", "restricted_agent")
	zendesk.SetBoolIfSet(body, inputs, "report_csv", "report_csv")
	zendesk.SetBoolIfSet(body, inputs, "suspended", "suspended")
	zendesk.SetBoolIfSet(body, inputs, "verified", "verified")
	zendesk.SetCSVIfPresent(body, inputs, "tags", "tags")
	if err := zendesk.SetJSONIfPresent(body, inputs, "user_fields", "user_fields"); err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}
	if err := zendesk.MergeAdditionalFields(body, inputs); err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}

	resp, err := zendesk.CreateResource(subdomain, auth, "/users.json", "user", body)
	if err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}
	out := zendesk.ResourceResult(resp, "user", "")
	out["tool_result"] = fmt.Sprintf("Created user %s", out["id"])
	return out, nil
}
