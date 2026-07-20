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
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Service Principal (keys)", Value: "keys"}, {Name: "Connect Azure", Value: "connect"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Azure Connection", Placeholder: "Pick a connected Azure account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"connect"}}},
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "keys"}}},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the service principal", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "keys"}}},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "The app needs a role (e.g. Reader) on the subscription or resource group", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "keys"}}},
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

	// The List call itself does NOT carry runtime power state — and the
	// resource-group List rejects $expand=instanceView outright (400) — so power
	// state is fetched per VM via InstanceView below.
	var vms []map[string]interface{}
	collect := func(vm *armcompute.VirtualMachine) {
		m := summariseVM(vm)
		// Best-effort power state: InstanceView is a per-VM read. The RG comes from
		// the scope when listing a group, else it is parsed from the VM's own ID
		// (subscription-wide list spans resource groups). A lookup failure just
		// leaves power_state absent rather than failing the whole listing.
		rg := auth.ResourceGroup
		if rg == "" {
			rg = resourceGroupFromID(compute.Str(vm.ID))
		}
		if name := compute.Str(vm.Name); rg != "" && name != "" {
			if iv, err := client.InstanceView(ctx, rg, name, nil); err == nil {
				if ps := powerState(&iv.VirtualMachineInstanceView); ps != "" {
					m["power_state"] = ps
				}
			}
		}
		vms = append(vms, m)
	}

	if auth.ResourceGroup != "" {
		pager := client.NewListPager(auth.ResourceGroup, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return compute.ErrorResult(auth.AzureError(err)), nil
			}
			for _, vm := range page.Value {
				collect(vm)
			}
		}
	} else {
		pager := client.NewListAllPager(nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return compute.ErrorResult(auth.AzureError(err)), nil
			}
			for _, vm := range page.Value {
				collect(vm)
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
		// provisioning_state is the ARM lifecycle (Succeeded/Creating/Failed) — kept
		// as a secondary field. The running/stopped/deallocated power_state (the
		// EC2 instance-state analogue an operator acts on) is added by Execute via a
		// per-VM InstanceView lookup, since the List response omits it.
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

// resourceGroupFromID extracts the resource-group name from an ARM resource ID
// (/subscriptions/{s}/resourceGroups/{rg}/providers/...), case-insensitively on
// the segment label. Empty if not found.
func resourceGroupFromID(id string) string {
	parts := strings.Split(id, "/")
	for i := 0; i+1 < len(parts); i++ {
		if strings.EqualFold(parts[i], "resourceGroups") {
			return parts[i+1]
		}
	}
	return ""
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
