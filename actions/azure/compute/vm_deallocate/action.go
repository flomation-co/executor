// Package azure_compute_vm_deallocate stops a Virtual Machine and releases its
// compute (the Azure equivalent of an EC2 stop — deallocate stops compute
// billing, unlike a plain power-off which keeps the VM allocated). The SDK
// deallocate is a long-running operation; Execute polls it to completion.
package azure_compute_vm_deallocate

import (
	"fmt"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/azure/compute"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure VM: Stop (Deallocate)"
	Description  = "Stop a Virtual Machine and release its compute so it stops incurring compute charges. Waits until the VM is deallocated."
	Website      = "https://www.flomation.co"
	Icon         = "azure+stop"
	Date         = "18/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Service Principal (keys)", Value: "keys"}, {Name: "Connect Azure", Value: "connect"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Azure Connection", Placeholder: "Pick a connected Azure account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"connect"}}},
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "keys"}}},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the service principal", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "keys"}}},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "The app needs a role (e.g. Virtual Machine Contributor) on the resource group", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "keys"}}},
	{Name: "subscription_id", Type: core.ConnectionTypeString, Label: "Subscription ID", Placeholder: "Azure subscription GUID", Required: true},
	{Name: "resource_group", Type: core.ConnectionTypeString, Label: "Resource Group", Placeholder: "my-resource-group", Required: true},
	{Name: "vm_name", Type: core.ConnectionTypeString, Label: "VM Name", Placeholder: "my-vm", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "VM Name"},
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
	name, err := compute.RequiredString("vm_name", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	client, err := auth.VMClient()
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	ctx := compute.Context()
	poller, err := client.BeginDeallocate(ctx, rg, name, nil)
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deallocated VM %q", name),
		"name":        name,
		"success":     true,
	}, nil
}
