// Package azure_compute_snapshot_create creates a snapshot of a managed disk
// (the Azure equivalent of EC2's create-snapshot). The snapshot is a full copy
// of the source disk; the SDK create is a long-running operation Execute polls
// to completion.
package azure_compute_snapshot_create

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
	Name         = "Azure Snapshot: Create"
	Description  = "Create a point-in-time snapshot of a managed disk. Waits until the snapshot is ready."
	Website      = "https://www.flomation.co"
	Icon         = "azure+copy"
	Date         = "18/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Required: true},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the service principal", Required: true},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "The app needs a role (e.g. Disk Snapshot Contributor) on the resource group", Required: true},
	{Name: "subscription_id", Type: core.ConnectionTypeString, Label: "Subscription ID", Placeholder: "Azure subscription GUID", Required: true},
	{Name: "resource_group", Type: core.ConnectionTypeString, Label: "Resource Group", Placeholder: "my-resource-group", Required: true},
	{Name: "snapshot_name", Type: core.ConnectionTypeString, Label: "Snapshot Name", Placeholder: "my-vm-osdisk-snap", Required: true},
	{Name: "location", Type: core.ConnectionTypeString, Label: "Location", Placeholder: "uksouth", Required: true},
	{Name: "source_disk_id", Type: core.ConnectionTypeString, Label: "Source Disk ID", Placeholder: "Full ARM ID of the managed disk — /subscriptions/.../disks/my-disk", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Snapshot Resource ID"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Snapshot Name"},
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
	name, err := compute.RequiredString("snapshot_name", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	location, err := compute.RequiredString("location", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	sourceDiskID, err := compute.RequiredString("source_disk_id", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}

	snap := armcompute.Snapshot{
		Location: to.Ptr(location),
		Properties: &armcompute.SnapshotProperties{
			CreationData: &armcompute.CreationData{
				CreateOption:     to.Ptr(armcompute.DiskCreateOptionCopy),
				SourceResourceID: to.Ptr(sourceDiskID),
			},
		},
	}

	client, err := auth.SnapshotsClient()
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	ctx := compute.Context()
	poller, err := client.BeginCreateOrUpdate(ctx, rg, name, snap, nil)
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	res, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created snapshot %q of disk", name),
		"id":          compute.Str(res.ID),
		"name":        name,
		"success":     true,
	}, nil
}
