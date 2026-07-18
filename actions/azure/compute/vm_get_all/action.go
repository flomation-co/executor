// Package azure_compute_vm_get_all lists Virtual Machines in a resource group,
// or across the whole subscription when the resource group is left blank. It is
// the reference implementation for the Azure Compute action template: an inline
// credential + scope input block, tool_result as the first output, and Execute
// delegating credential/client construction to the shared compute package.
package azure_compute_vm_get_all

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/azure/compute"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure VM: List Virtual Machines"
	Description  = "List Virtual Machines in a resource group (or across the whole subscription), with their power state (running/deallocated), size, location, OS and tags."
	Website      = "https://www.flomation.co"
	Icon         = "azure+list"
	Date         = "18/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Required: true},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the service principal", Required: true},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "The app needs a role (e.g. Reader) on the subscription or resource group", Required: true},
	{Name: "subscription_id", Type: core.ConnectionTypeString, Label: "Subscription ID", Placeholder: "Azure subscription GUID", Required: true},
	{Name: "resource_group", Type: core.ConnectionTypeString, Label: "Resource Group", Placeholder: "Leave blank to list across the whole subscription (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "virtual_machines", Type: core.ConnectionTypeObject, Label: "Virtual Machines"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := compute.GetAuth(inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	client, err := auth.VMClient()
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	ctx := compute.Context()

	var vms []map[string]interface{}
	if auth.ResourceGroup != "" {
		// Expand instanceView so each VM carries its runtime power state, not just
		// the ARM provisioning state — the resource-group List supports this expand
		// directly (unlike the subscription-wide List, which uses StatusOnly below).
		pager := client.NewListPager(auth.ResourceGroup, &armcompute.VirtualMachinesClientListOptions{
			Expand: to.Ptr(armcompute.ExpandTypeForListVMsInstanceView),
		})
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return compute.ErrorResult(auth.AzureError(err)), nil
			}
			for _, vm := range page.Value {
				vms = append(vms, summariseVM(vm))
			}
		}
	} else {
		// StatusOnly fetches the runtime status (power state) of every VM in the
		// subscription — the subscription List's way to populate instanceView.
		pager := client.NewListAllPager(&armcompute.VirtualMachinesClientListAllOptions{
			StatusOnly: to.Ptr("true"),
		})
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return compute.ErrorResult(auth.AzureError(err)), nil
			}
			for _, vm := range page.Value {
				vms = append(vms, summariseVM(vm))
			}
		}
	}

	scope := "the subscription"
	if auth.ResourceGroup != "" {
		scope = fmt.Sprintf("resource group %q", auth.ResourceGroup)
	}
	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Found %d virtual machine(s) in %s", len(vms), scope),
		"virtual_machines": vms,
		"count":            len(vms),
		"success":          true,
	}, nil
}

// summariseVM flattens the SDK VM into a compact, JSON-friendly map.
func summariseVM(vm *armcompute.VirtualMachine) map[string]interface{} {
	m := map[string]interface{}{
		"id":       compute.Str(vm.ID),
		"name":     compute.Str(vm.Name),
		"location": compute.Str(vm.Location),
	}
	if p := vm.Properties; p != nil {
		if p.HardwareProfile != nil && p.HardwareProfile.VMSize != nil {
			m["vm_size"] = string(*p.HardwareProfile.VMSize)
		}
		// power_state is the running/stopped/deallocated runtime state — the field
		// an operator actually acts on (the EC2 instance-state analogue). It is
		// distinct from provisioning_state (the ARM lifecycle: Succeeded/Creating/
		// Failed), which is kept as a secondary field. power_state is only present
		// when the caller expanded instanceView (this action does).
		m["power_state"] = powerState(p.InstanceView)
		m["provisioning_state"] = compute.Str(p.ProvisioningState)
		if p.StorageProfile != nil && p.StorageProfile.OSDisk != nil && p.StorageProfile.OSDisk.OSType != nil {
			m["os_type"] = string(*p.StorageProfile.OSDisk.OSType)
		}
	}
	tags := map[string]string{}
	for k, v := range vm.Tags {
		tags[k] = compute.Str(v)
	}
	m["tags"] = tags
	return m
}

// powerState pulls the runtime power state ("running", "deallocated",
// "stopped", ...) out of the instance view's status list, where it appears as a
// "PowerState/<state>" code. Empty when the instance view was not expanded or
// carries no power status.
func powerState(iv *armcompute.VirtualMachineInstanceView) string {
	if iv == nil {
		return ""
	}
	for _, s := range iv.Statuses {
		if code := compute.Str(s.Code); strings.HasPrefix(code, "PowerState/") {
			return strings.TrimPrefix(code, "PowerState/")
		}
	}
	return ""
}
