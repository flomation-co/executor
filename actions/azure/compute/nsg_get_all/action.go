// Package azure_compute_nsg_get_all lists Network Security Groups in a resource
// group, or across the whole subscription when the resource group is left
// blank (the Azure equivalent of describing EC2 security groups).
package azure_compute_nsg_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/azure/compute"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure NSG: List Security Groups"
	Description  = "List Network Security Groups in a resource group (or across the whole subscription), with their location, rule count and tags."
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
	{Name: "network_security_groups", Type: core.ConnectionTypeObject, Label: "Network Security Groups"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := compute.GetAuth(inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	client, err := auth.NSGClient()
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	ctx := compute.Context()

	var groups []map[string]interface{}
	collect := func(page []*armnetwork.SecurityGroup) {
		for _, nsg := range page {
			groups = append(groups, summariseNSG(nsg))
		}
	}
	if auth.ResourceGroup != "" {
		pager := client.NewListPager(auth.ResourceGroup, nil)
		for pager.More() {
			p, err := pager.NextPage(ctx)
			if err != nil {
				return compute.ErrorResult(auth.AzureError(err)), nil
			}
			collect(p.Value)
		}
	} else {
		pager := client.NewListAllPager(nil)
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
		"tool_result":             fmt.Sprintf("Found %d network security group(s) in %s", len(groups), scope),
		"network_security_groups": groups,
		"count":                   len(groups),
		"success":                 true,
	}, nil
}

func summariseNSG(nsg *armnetwork.SecurityGroup) map[string]interface{} {
	m := map[string]interface{}{
		"id":       compute.Str(nsg.ID),
		"name":     compute.Str(nsg.Name),
		"location": compute.Str(nsg.Location),
	}
	if p := nsg.Properties; p != nil {
		m["rule_count"] = len(p.SecurityRules)
		if p.ProvisioningState != nil {
			m["provisioning_state"] = string(*p.ProvisioningState)
		}
	}
	tags := map[string]string{}
	for k, v := range nsg.Tags {
		tags[k] = compute.Str(v)
	}
	m["tags"] = tags
	return m
}
