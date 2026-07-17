package azure_entra_group_create

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	entra "flomation.app/automate/executor/actions/azure/entra"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Entra ID: Create Group"
	Description  = "Create a Security or Microsoft 365 group. Setting a Dynamic Membership Rule makes the group dynamic (rule processing is switched on). Requires the Group.ReadWrite.All application permission."
	Website      = "https://www.flomation.co"
	Icon         = "azure+plus"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Required: true},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the app registration", Required: true},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "App registration ▸ Certificates & secrets — the secret Value, not its ID", Required: true},
	{Name: "graph_endpoint", Type: core.ConnectionTypeString, Label: "Graph Endpoint", Placeholder: "https://graph.microsoft.com — override for sovereign clouds (e.g. https://graph.microsoft.us)"},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Sales Team (max 256 characters)", Required: true},
	{Name: "mail_nickname", Type: core.ConnectionTypeString, Label: "Mail Nickname", Placeholder: "sales-team — local part only, no @ (max 64 characters)", Required: true},
	{
		Name:  "group_type",
		Type:  core.ConnectionTypeString,
		Label: "Group Type",
		Options: []core.ConnectionOption{
			{Name: "Security", Value: "security"},
			{Name: "Microsoft 365", Value: "unified"},
		},
	},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description"},
	{
		Name:  "visibility",
		Type:  core.ConnectionTypeString,
		Label: "Visibility",
		Options: []core.ConnectionOption{
			{Name: "Default (tenant setting)", Value: ""},
			{Name: "Public", Value: "Public"},
			{Name: "Private", Value: "Private"},
		},
	},
	{Name: "membership_rule", Type: core.ConnectionTypeText, Label: "Dynamic Membership Rule", Placeholder: `(user.department -eq "Sales") — setting a rule makes the group dynamic and turns rule processing on`},
	{Name: "owners", Type: core.ConnectionTypeString, Label: "Owners", Placeholder: "Comma-separated user object IDs to set as owners"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"isAssignableToRole":true,"preferredDataLocation":"EUR"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Group ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Group"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := entra.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	displayName, err := entra.RequiredString("display_name", inputs)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	nickname, err := entra.RequiredString("mail_nickname", inputs)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	if err := entra.ValidateDisplayName(displayName); err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	if err := entra.ValidateMailNickname(nickname); err != nil {
		return entra.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{
		"displayName":  displayName,
		"mailNickname": nickname,
	}
	groupTypes := []interface{}{}
	// Graph requires mailEnabled and securityEnabled explicitly, even though
	// the group type implies them — an unset dropdown means Security.
	if entra.OptionalString("group_type", inputs) == "unified" {
		groupTypes = append(groupTypes, "Unified")
		body["mailEnabled"] = true
		body["securityEnabled"] = false
	} else {
		body["mailEnabled"] = false
		body["securityEnabled"] = true
	}
	entra.SetIfPresent(body, inputs, "description", "description")
	entra.SetIfPresent(body, inputs, "visibility", "visibility")
	if rule := entra.OptionalString("membership_rule", inputs); rule != "" {
		groupTypes = append(groupTypes, "DynamicMembership")
		body["membershipRule"] = rule
		body["membershipRuleProcessingState"] = "On"
	}
	if len(groupTypes) > 0 {
		body["groupTypes"] = groupTypes
	}
	if owners := entra.SplitCommaList(entra.OptionalString("owners", inputs)); len(owners) > 0 {
		binds := []interface{}{}
		for _, id := range owners {
			binds = append(binds, auth.BaseURL()+"/users/"+url.PathEscape(id))
		}
		body["owners@odata.bind"] = binds
	}
	if err := entra.MergeAdditionalFields(body, inputs); err != nil {
		return entra.ErrorResult(err.Error()), nil
	}

	resp, err := entra.ExecuteAPI(flow, auth, "POST", "/groups", body)
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
	out := entra.ResourceResult(obj, "")
	// Graph replicates a new group asynchronously, so the id above 404s for a
	// few seconds. Wait for it here rather than hand a downstream "add members"
	// step an id that is not yet real.
	if id, ok := out["id"].(string); ok {
		entra.WaitUntilReadable(flow, auth, "/groups", id)
	}
	out["tool_result"] = fmt.Sprintf("Created group %s (%s)", displayName, out["id"])
	return out, nil
}
