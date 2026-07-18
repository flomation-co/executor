// Package azure_compute_nsg_create creates a Network Security Group (the Azure
// equivalent of an EC2 security group). A new NSG starts with Azure's default
// rules; add inbound rules with the "Add Inbound Rule" action. The SDK create
// is a long-running operation Execute polls to completion.
package azure_compute_nsg_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/azure/compute"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure NSG: Create Security Group"
	Description  = "Create a Network Security Group. It starts with Azure's default rules; add your own with the Add Inbound Rule action."
	Website      = "https://www.flomation.co"
	Icon         = "azure+lock"
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
	{Name: "location", Type: core.ConnectionTypeString, Label: "Location", Placeholder: "uksouth", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "NSG Resource ID"},
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
	location, err := compute.RequiredString("location", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	client, err := auth.NSGClient()
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	ctx := compute.Context()
	poller, err := client.BeginCreateOrUpdate(ctx, rg, name, armnetwork.SecurityGroup{Location: to.Ptr(location)}, nil)
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	res, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created security group %q in %s", name, location),
		"id":          compute.Str(res.ID),
		"name":        name,
		"success":     true,
	}, nil
}
