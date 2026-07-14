// Package infrastructure_awx_me reports which AWX user a credential belongs to.
//
// GET {root}me/ is a PAGINATED LIST, not an object: it answers
// {"count":1,"results":[{…}]} and the user is results[0]. awx.FetchMe unwraps it.
// Reaching into the body for a "username" key would silently yield nothing.
//
// This is also the cheapest possible proof that a credential works: unlike ping/,
// me/ requires authentication on every deployment.
package infrastructure_awx_me

import (
	"fmt"

	core "flomation.app/automate/executor"
	awx "flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "AWX: Get Current User"
	Description  = "Show which AWX user your credential belongs to, and whether they are a superuser."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+user"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// ---- AUTH BLOCK (identical on every AWX action; see awx.AuthInputs) ----
	{Name: "awx_url", Type: core.ConnectionTypeString, Label: "AWX / AAP URL", Placeholder: "https://awx.example.com — your AWX or Ansible Automation Platform address", Required: true},
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{
		{Name: "API Token (recommended)", Value: "token"},
		{Name: "Username & Password", Value: "basic"},
	}},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "AWX ▸ your user ▸ Tokens ▸ Add, Application blank, Scope = Write. Shown once — copy it then.", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "token"}}},
	{Name: "awx_username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "your AWX username", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"basic"}}},
	{Name: "awx_password", Type: core.ConnectionTypeSecret, Label: "Password", Placeholder: "your AWX password — note some AWX installs disable password authentication", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"basic"}}},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure TLS", Placeholder: "Skip certificate verification — only for a self-hosted AWX with a self-signed certificate"},
	{Name: "api_prefix", Type: core.ConnectionTypeString, Label: "API Path Prefix (advanced)", Placeholder: "Leave blank — detected automatically. Only set this if support asks (e.g. /api/controller/v2/)."},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "User ID"},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email"},
	{Name: "is_superuser", Type: core.ConnectionTypeBoolean, Label: "Superuser"},
	{Name: "is_system_auditor", Type: core.ConnectionTypeBoolean, Label: "System Auditor"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "User"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := awx.GetAuth(inputs)
	if err != nil {
		return nil, err // the ONLY hard failure: the node is mis-configured
	}

	ctx, cancel := awx.Context()
	defer cancel()

	me, err := awx.FetchMe(ctx, auth)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	username := awx.StringField(me, "username")
	superuser := awx.BoolField(me, "is_superuser")
	auditor := awx.BoolField(me, "is_system_auditor")

	role := "a normal user"
	switch {
	case superuser:
		role = "a superuser"
	case auditor:
		role = "a system auditor (read-only across AWX)"
	}
	summary := fmt.Sprintf("This credential belongs to %s — %s", username, role)

	out := awx.ObjectResult(me, summary)
	out["username"] = username
	out["email"] = awx.StringField(me, "email")
	out["is_superuser"] = superuser
	out["is_system_auditor"] = auditor
	return out, nil
}
