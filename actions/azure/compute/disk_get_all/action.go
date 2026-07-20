// Package azure_compute_disk_get_all lists managed disks in a resource group,
// or across the whole subscription when the resource group is left blank (the
// Azure equivalent of describing EC2 volumes).
package azure_compute_disk_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/azure/compute"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Disk: List Managed Disks"
	Description  = "List managed disks in a resource group (or across the whole subscription), with size, SKU and the VM each is attached to."
	Website      = "https://www.flomation.co"
	Icon         = "azure+database"
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
	{Name: "disks", Type: core.ConnectionTypeObject, Label: "Disks"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := compute.GetAuth(inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	client, err := auth.DisksClient()
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	ctx := compute.Context()

	var disks []map[string]interface{}
	collect := func(page []*armcompute.Disk) {
		for _, d := range page {
			disks = append(disks, summariseDisk(d))
		}
	}
	if auth.ResourceGroup != "" {
		pager := client.NewListByResourceGroupPager(auth.ResourceGroup, nil)
		for pager.More() {
			p, err := pager.NextPage(ctx)
			if err != nil {
				return compute.ErrorResult(auth.AzureError(err)), nil
			}
			collect(p.Value)
		}
	} else {
		pager := client.NewListPager(nil)
		for pager.More() {
			p, err := pager.NextPage(ctx)
			if err != nil {
				return compute.ErrorResult(auth.AzureError(err)), nil
			}
			collect(p.Value)
		}
	}

	scope := "the subscription"
	if auth.ResourceGroup != "" {
		scope = fmt.Sprintf("resource group %q", auth.ResourceGroup)
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d managed disk(s) in %s", len(disks), scope),
		"disks":       disks,
		"count":       len(disks),
		"success":     true,
	}, nil
}

func summariseDisk(d *armcompute.Disk) map[string]interface{} {
	m := map[string]interface{}{
		"id":         compute.Str(d.ID),
		"name":       compute.Str(d.Name),
		"location":   compute.Str(d.Location),
		"managed_by": compute.Str(d.ManagedBy), // the VM the disk is attached to, if any
	}
	if d.SKU != nil && d.SKU.Name != nil {
		m["sku"] = string(*d.SKU.Name)
	}
	if p := d.Properties; p != nil {
		if p.DiskSizeGB != nil {
			m["disk_size_gb"] = *p.DiskSizeGB
		}
		if p.DiskState != nil {
			m["disk_state"] = string(*p.DiskState)
		}
	}
	return m
}
