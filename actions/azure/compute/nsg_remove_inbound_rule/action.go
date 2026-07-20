// Package azure_compute_nsg_remove_inbound_rule removes a named security rule
// from a Network Security Group — the Azure equivalent of EC2's
// revoke-security-group-ingress. The SDK delete is a long-running operation
// Execute polls to completion.
package azure_compute_nsg_remove_inbound_rule

import (
	"fmt"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/azure/compute"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure NSG: Remove Inbound Rule"
	Description  = "Remove a named rule from a Network Security Group."
	Website      = "https://www.flomation.co"
	Icon         = "azure+minus"
	Date         = "18/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Service Principal (keys)", Value: "keys"}, {Name: "Connect Azure", Value: "connect"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Azure Connection", Placeholder: "Pick a connected Azure account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"connect"}}},
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "keys"}}},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the service principal", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "keys"}}},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "The app needs a role (e.g. Network Contributor) on the resource group", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "keys"}}},
	{Name: "subscription_id", Type: core.ConnectionTypeString, Label: "Subscription ID", Placeholder: "Azure subscription GUID", Required: true},
	{Name: "resource_group", Type: core.ConnectionTypeString, Label: "Resource Group", Placeholder: "my-resource-group", Required: true},
	{Name: "nsg_name", Type: core.ConnectionTypeString, Label: "Security Group Name", Placeholder: "my-nsg", Required: true},
	{Name: "rule_name", Type: core.ConnectionTypeString, Label: "Rule Name", Placeholder: "allow-ssh", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Rule Name"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := compute.GetAuth(inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	rg, err := auth.RequiredResourceGroup()
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	nsgName, err := compute.RequiredString("nsg_name", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	ruleName, err := compute.RequiredString("rule_name", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	client, err := auth.SecurityRulesClient()
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	ctx := compute.Context()
	poller, err := client.BeginDelete(ctx, rg, nsgName, ruleName, nil)
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Removed rule %q from %q", ruleName, nsgName),
		"name":        ruleName,
		"success":     true,
	}, nil
}
