// Package infrastructure_awx_credential_delete permanently deletes a credential
// from AWX.
//
// DESTRUCTIVE, and quietly far-reaching: AWX's foreign keys are SET_NULL / M2M
// detach, so deleting a credential does not delete the job templates or projects
// that use it — it silently DETACHES it from them, and they then fail at their
// next run with an authentication error nobody connects to this action. Hence
// confirm_destructive, which fails closed.
//
// AWX-managed credentials (the built-ins) answer 403 {"detail": "Deletion not
// allowed for managed credentials"}. That is a permanent property of the object,
// not a permissions problem with the token, and the generic 403 message ("this
// AWX user does not have permission") would send the operator hunting for a role
// they can never be granted — so it is reworded here.
package infrastructure_awx_credential_delete

import (
	"fmt"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Delete Credential"
	Description  = "Permanently delete a credential from AWX. Job templates that use it will stop working."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+trash"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
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

	{Name: "credential_id", Type: core.ConnectionTypeString, Label: "Credential", Placeholder: "The credential to delete — permanently", Required: true},
	{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Label: "Confirm Destructive Action", Placeholder: "This permanently changes AWX state. Tick to allow, or bind a variable such as ${var.approved}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Credential ID"},
	{Name: "deleted", Type: core.ConnectionTypeBoolean, Label: "Deleted"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := awx.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	credentialID, err := awx.RequiredInt("credential_id", "Credential", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if err := awx.ConfirmDestructive(inputs, fmt.Sprintf("permanently delete AWX credential %d", credentialID)); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	ctx, cancel := awx.Context()
	defer cancel()

	resp, err := awx.Do(ctx, auth, http.MethodDelete, fmt.Sprintf("credentials/%d/", credentialID), nil)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if resp.StatusCode == http.StatusForbidden && strings.Contains(strings.ToLower(string(resp.Body)), "managed") {
		return awx.ErrorResult(fmt.Sprintf(
			"Credential %d is managed by AWX itself and can never be deleted — not by any user, however many permissions they have. Only credentials someone created can be removed.",
			credentialID)), nil
	}
	if err := awx.CheckResponse(auth, resp, http.StatusNoContent); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	return awx.SuccessResult(
		fmt.Sprintf("Deleted credential %d. Any job template or project that used it has been detached from it and will fail to authenticate until a new credential is attached.", credentialID),
		map[string]interface{}{
			"id":      awx.IDString(credentialID),
			"deleted": true,
		}), nil
}
