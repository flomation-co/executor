// Package azure_compute_image_get_all lists custom managed images in a resource
// group, or across the whole subscription when the resource group is left blank
// (the Azure equivalent of describing your own EC2 AMIs). Marketplace images
// are a separate catalogue and are chosen by publisher/offer/sku on VM create.
package azure_compute_image_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/azure/compute"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Image: List Custom Images"
	Description  = "List custom managed images in a resource group (or across the whole subscription). Marketplace images are selected by publisher/offer/sku when creating a VM."
	Website      = "https://www.flomation.co"
	Icon         = "azure+image"
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
	{Name: "images", Type: core.ConnectionTypeObject, Label: "Images"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := compute.GetAuth(inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	client, err := auth.ImagesClient()
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	ctx := compute.Context()

	var images []map[string]interface{}
	collect := func(page []*armcompute.Image) {
		for _, img := range page {
			images = append(images, summariseImage(img))
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
		"tool_result": fmt.Sprintf("Found %d custom image(s) in %s", len(images), scope),
		"images":      images,
		"count":       len(images),
		"success":     true,
	}, nil
}

func summariseImage(img *armcompute.Image) map[string]interface{} {
	m := map[string]interface{}{
		"id":       compute.Str(img.ID),
		"name":     compute.Str(img.Name),
		"location": compute.Str(img.Location),
	}
	if p := img.Properties; p != nil {
		m["provisioning_state"] = compute.Str(p.ProvisioningState)
	}
	return m
}
