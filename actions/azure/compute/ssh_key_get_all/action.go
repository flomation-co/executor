// Package azure_compute_ssh_key_get_all lists SSH public keys stored in a
// resource group, or across the whole subscription when the resource group is
// left blank (the Azure equivalent of describing EC2 key pairs). Azure stores
// only the PUBLIC key; there is no private-key material to return.
package azure_compute_ssh_key_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/azure/compute"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure SSH Key: List"
	Description  = "List stored SSH public keys in a resource group (or across the whole subscription). Only the public key is stored by Azure."
	Website      = "https://www.flomation.co"
	Icon         = "azure+key"
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
	{Name: "ssh_keys", Type: core.ConnectionTypeObject, Label: "SSH Public Keys"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := compute.GetAuth(inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	client, err := auth.SSHKeysClient()
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	ctx := compute.Context()

	var keys []map[string]interface{}
	collect := func(page []*armcompute.SSHPublicKeyResource) {
		for _, k := range page {
			keys = append(keys, summariseSSHKey(k))
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
		pager := client.NewListBySubscriptionPager(nil)
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
		"tool_result": fmt.Sprintf("Found %d SSH public key(s) in %s", len(keys), scope),
		"ssh_keys":    keys,
		"count":       len(keys),
		"success":     true,
	}, nil
}

func summariseSSHKey(k *armcompute.SSHPublicKeyResource) map[string]interface{} {
	m := map[string]interface{}{
		"id":       compute.Str(k.ID),
		"name":     compute.Str(k.Name),
		"location": compute.Str(k.Location),
	}
	if p := k.Properties; p != nil {
		m["public_key"] = compute.Str(p.PublicKey)
	}
	return m
}
