// Package azure_compute_snapshot_get_all lists managed-disk snapshots in a
// resource group, or across the whole subscription when the resource group is
// left blank (the Azure equivalent of describing EC2 snapshots).
package azure_compute_snapshot_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/azure/compute"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Snapshot: List"
	Description  = "List managed-disk snapshots in a resource group (or across the whole subscription), with size, state and location."
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
	{Name: "snapshots", Type: core.ConnectionTypeObject, Label: "Snapshots"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := compute.GetAuth(inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	client, err := auth.SnapshotsClient()
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	ctx := compute.Context()

	var snaps []map[string]interface{}
	collect := func(page []*armcompute.Snapshot) {
		for _, s := range page {
			snaps = append(snaps, summariseSnapshot(s))
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
		"tool_result": fmt.Sprintf("Found %d snapshot(s) in %s", len(snaps), scope),
		"snapshots":   snaps,
		"count":       len(snaps),
		"success":     true,
	}, nil
}

func summariseSnapshot(s *armcompute.Snapshot) map[string]interface{} {
	m := map[string]interface{}{
		"id":       compute.Str(s.ID),
		"name":     compute.Str(s.Name),
		"location": compute.Str(s.Location),
	}
	if p := s.Properties; p != nil {
		if p.DiskSizeGB != nil {
			m["disk_size_gb"] = *p.DiskSizeGB
		}
		m["provisioning_state"] = compute.Str(p.ProvisioningState)
		m["time_created"] = ""
		if p.TimeCreated != nil {
			m["time_created"] = p.TimeCreated.UTC().Format("2006-01-02T15:04:05Z")
		}
	}
	return m
}
