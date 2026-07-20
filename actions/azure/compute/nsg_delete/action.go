// Package azure_compute_nsg_delete deletes a Network Security Group (the Azure
// equivalent of deleting an EC2 security group). Azure refuses if the NSG is
// still attached to a subnet or NIC — that surfaces as a clean error. The SDK
// delete is a long-running operation Execute polls to completion.
package azure_compute_nsg_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/azure/compute"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure NSG: Delete Security Group"
	Description  = "Delete a Network Security Group. Azure refuses if it is still attached to a subnet or network interface."
	Website      = "https://www.flomation.co"
	Icon         = "azure+trash"
	Date         = "18/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Required: true},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the service principal", Required: true},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "The app needs a role (e.g. Network Contributor) on the resource group", Required: true},
	{Name: "subscription_id", Type: core.ConnectionTypeString, Label: "Subscription ID", Placeholder: "Azure subscription GUID", Required: true},
	{Name: "resource_group", Type: core.ConnectionTypeString, Label: "Resource Group", Placeholder: "my-resource-group", Required: true},
	{Name: "nsg_name", Type: core.ConnectionTypeString, Label: "Security Group Name", Placeholder: "my-nsg", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Security Group Name"},
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
	name, err := compute.RequiredString("nsg_name", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	client, err := auth.NSGClient()
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	ctx := compute.Context()
	poller, err := client.BeginDelete(ctx, rg, name, nil)
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleted security group %q", name),
		"name":        name,
		"success":     true,
	}, nil
}
