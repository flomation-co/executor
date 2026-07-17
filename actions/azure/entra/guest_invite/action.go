package azure_entra_guest_invite

import (
	"fmt"

	core "flomation.app/automate/executor"
	entra "flomation.app/automate/executor/actions/azure/entra"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Entra ID: Invite Guest"
	Description  = "Invite an external (B2B) guest user by email. A guest account is created immediately; the redeem link is returned so you can send it yourself if you turn the invitation email off. Requires the User.Invite.All application permission."
	Website      = "https://www.flomation.co"
	Icon         = "azure+envelope"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Required: true},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the app registration", Required: true},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "App registration ▸ Certificates & secrets — the secret Value, not its ID", Required: true},
	{Name: "graph_endpoint", Type: core.ConnectionTypeString, Label: "Graph Endpoint", Placeholder: "https://graph.microsoft.com — override for sovereign clouds (e.g. https://graph.microsoft.us)"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email Address", Placeholder: "guest@partner.com", Required: true},
	{Name: "redirect_url", Type: core.ConnectionTypeString, Label: "Redirect URL", Placeholder: "https://myapplications.microsoft.com (default) — where the guest lands after redeeming"},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Shown in the directory and the invitation email"},
	{Name: "send_invitation", Type: core.ConnectionTypeBoolean, Label: "Send Invitation Email", Placeholder: "On (default): Microsoft emails the invite; off: use the returned redeem URL yourself", Value: true},
	{Name: "message", Type: core.ConnectionTypeText, Label: "Custom Message", Placeholder: "Personal note included in the invitation email"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Invitation ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Invitation"},
	{Name: "invite_redeem_url", Type: core.ConnectionTypeString, Label: "Invite Redeem URL"},
	{Name: "invited_user_id", Type: core.ConnectionTypeString, Label: "Invited User ID"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := entra.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	email, err := entra.RequiredString("email", inputs)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}

	redirect := entra.OptionalString("redirect_url", inputs)
	if redirect == "" {
		redirect = "https://myapplications.microsoft.com"
	}
	body := map[string]interface{}{
		"invitedUserEmailAddress": email,
		"inviteRedirectUrl":       redirect,
		"sendInvitationMessage":   entra.BoolOrDefault("send_invitation", inputs, true),
	}
	entra.SetIfPresent(body, inputs, "invitedUserDisplayName", "display_name")
	if msg := entra.OptionalString("message", inputs); msg != "" {
		body["invitedUserMessageInfo"] = map[string]interface{}{"customizedMessageBody": msg}
	}

	resp, err := entra.ExecuteAPI(flow, auth, "POST", "/invitations", body)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	if err := entra.CheckResponse(resp); err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	obj, err := entra.Decode(resp)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}

	out := entra.ResourceResult(obj, fmt.Sprintf("Invited guest %s", email))
	redeemURL, _ := obj["inviteRedeemUrl"].(string)
	out["invite_redeem_url"] = redeemURL
	invitedUserID := ""
	if u, ok := obj["invitedUser"].(map[string]interface{}); ok {
		invitedUserID, _ = u["id"].(string)
	}
	out["invited_user_id"] = invitedUserID
	return out, nil
}
