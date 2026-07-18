// Package azure_compute_vm_create creates a Virtual Machine (the Azure
// equivalent of EC2 run_instances). Azure VM creation needs more wiring than
// EC2's: a VM must attach to an EXISTING network interface, and its OS comes
// from a marketplace image reference (publisher/offer/sku/version). Those are
// the required inputs here; the SDK create is a long-running operation Execute
// polls to completion.
package azure_compute_vm_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/azure/compute"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure VM: Create"
	Description  = "Create a Virtual Machine from a marketplace image, attached to an existing network interface. Waits until the VM is provisioned."
	Website      = "https://www.flomation.co"
	Icon         = "azure+plus"
	Date         = "18/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Required: true},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the service principal", Required: true},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "The app needs a role (e.g. Virtual Machine Contributor) on the resource group", Required: true},
	{Name: "subscription_id", Type: core.ConnectionTypeString, Label: "Subscription ID", Placeholder: "Azure subscription GUID", Required: true},
	{Name: "resource_group", Type: core.ConnectionTypeString, Label: "Resource Group", Placeholder: "my-resource-group", Required: true},
	{Name: "vm_name", Type: core.ConnectionTypeString, Label: "VM Name", Placeholder: "my-vm (also used as the computer name)", Required: true},
	{Name: "location", Type: core.ConnectionTypeString, Label: "Location", Placeholder: "uksouth", Required: true},
	{Name: "vm_size", Type: core.ConnectionTypeString, Label: "VM Size", Placeholder: "Standard_B1s", Required: true},
	{Name: "image_publisher", Type: core.ConnectionTypeString, Label: "Image Publisher", Placeholder: "Canonical", Required: true},
	{Name: "image_offer", Type: core.ConnectionTypeString, Label: "Image Offer", Placeholder: "0001-com-ubuntu-server-jammy", Required: true},
	{Name: "image_sku", Type: core.ConnectionTypeString, Label: "Image SKU", Placeholder: "22_04-lts-gen2", Required: true},
	{Name: "image_version", Type: core.ConnectionTypeString, Label: "Image Version", Placeholder: "latest (default)"},
	{Name: "admin_username", Type: core.ConnectionTypeString, Label: "Admin Username", Placeholder: "azureuser", Required: true},
	{Name: "admin_password", Type: core.ConnectionTypeSecret, Label: "Admin Password", Placeholder: "Must meet Azure complexity rules (12+ chars, 3 of 4 classes)", Required: true},
	{Name: "network_interface_id", Type: core.ConnectionTypeString, Label: "Network Interface ID", Placeholder: "Full ARM ID of an existing NIC — /subscriptions/.../networkInterfaces/my-nic", Required: true},
	{Name: "os_disk_storage_type", Type: core.ConnectionTypeString, Label: "OS Disk Storage Type", Placeholder: "Standard_LRS (default) — or StandardSSD_LRS, Premium_LRS"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "VM Resource ID"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "VM Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Virtual Machine"},
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
	// Required fields, each naming itself on failure.
	name, err := compute.RequiredString("vm_name", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	location, err := compute.RequiredString("location", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	size, err := compute.RequiredString("vm_size", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	publisher, err := compute.RequiredString("image_publisher", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	offer, err := compute.RequiredString("image_offer", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	sku, err := compute.RequiredString("image_sku", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	adminUser, err := compute.RequiredString("admin_username", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	// Read the password UNTRIMMED — Azure permits leading/trailing whitespace in
	// admin passwords, so trimming (as RequiredString does for identifiers) would
	// silently create the VM with a different credential than the operator set.
	// This mirrors how the client secret is read untrimmed in GetAuth.
	adminPass := compute.OptionalString("admin_password", inputs)
	if adminPass == "" {
		return compute.ErrorResult("admin password is required"), nil
	}
	// Azure requires 12–123 chars plus complexity; a fast length check here beats
	// a slow server-side 400 for the common too-short case. Complexity is left to
	// Azure to avoid rejecting passwords its own rules would accept.
	if len(adminPass) < 12 {
		return compute.ErrorResult("admin password must be at least 12 characters (Azure requirement)"), nil
	}
	nicID, err := compute.RequiredString("network_interface_id", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	version := compute.OptionalString("image_version", inputs)
	if version == "" {
		version = "latest"
	}
	diskType := compute.OptionalString("os_disk_storage_type", inputs)
	if diskType == "" {
		diskType = "Standard_LRS"
	}

	vm := armcompute.VirtualMachine{
		Location: to.Ptr(location),
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes(size)),
			},
			StorageProfile: &armcompute.StorageProfile{
				ImageReference: &armcompute.ImageReference{
					Publisher: to.Ptr(publisher),
					Offer:     to.Ptr(offer),
					SKU:       to.Ptr(sku),
					Version:   to.Ptr(version),
				},
				OSDisk: &armcompute.OSDisk{
					CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
					ManagedDisk: &armcompute.ManagedDiskParameters{
						StorageAccountType: to.Ptr(armcompute.StorageAccountTypes(diskType)),
					},
				},
			},
			OSProfile: &armcompute.OSProfile{
				ComputerName:  to.Ptr(name),
				AdminUsername: to.Ptr(adminUser),
				AdminPassword: to.Ptr(adminPass),
			},
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
					{ID: to.Ptr(nicID)},
				},
			},
		},
	}

	client, err := auth.VMClient()
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	ctx := compute.Context()
	poller, err := client.BeginCreateOrUpdate(ctx, rg, name, vm, nil)
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	res, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}

	result := map[string]interface{}{
		"id":       compute.Str(res.ID),
		"name":     compute.Str(res.Name),
		"location": compute.Str(res.Location),
	}
	if res.Properties != nil {
		result["provisioning_state"] = compute.Str(res.Properties.ProvisioningState)
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created VM %q (%s) in %s", name, size, location),
		"id":          compute.Str(res.ID),
		"name":        name,
		"result":      result,
		"success":     true,
	}, nil
}
