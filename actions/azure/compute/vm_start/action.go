// Package azure_compute_vm_start starts a stopped/deallocated Virtual Machine.
// The SDK start is a long-running operation; Execute polls it to completion so
// the flow only continues once the VM is actually running.
package azure_compute_vm_start

import (
	"fmt"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/azure/compute"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure VM: Start"
	Description  = "Start a stopped or deallocated Virtual Machine and wait until it is running."
	Website      = "https://www.flomation.co"
	Icon         = "azure+play"
	Date         = "18/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Required: true},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the service principal", Required: true},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "The app needs a role (e.g. Virtual Machine Contributor) on the resource group", Required: true},
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
	poller, err := client.BeginStart(ctx, rg, name, nil)
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Started VM %q", name),
		"name":        name,
		"success":     true,
	}, nil
}
