// Package azure_compute_resource_tag_set adds or updates tags on any Azure
// resource by its ARM ID (the Azure equivalent of EC2's create-tags). It uses
// the Merge operation, so existing tags are kept and only the supplied keys are
// added or overwritten — matching EC2's additive create-tags. No resource group
// input is needed: the resource ID is the full scope.
package azure_compute_resource_tag_set

import (
	"fmt"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/azure/compute"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Resource: Set Tags"
	Description  = "Add or update tags on any Azure resource by its ARM ID. Existing tags are preserved; only the supplied keys are added or overwritten."
	Website      = "https://www.flomation.co"
	Icon         = "azure+bookmark"
	Date         = "18/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Service Principal (keys)", Value: "keys"}, {Name: "Connect Azure", Value: "connect"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Azure Connection", Placeholder: "Pick a connected Azure account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"connect"}}},
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "keys"}}},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the service principal", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "keys"}}},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "The app needs a role (e.g. Tag Contributor) on the resource", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "keys"}}},
	{Name: "subscription_id", Type: core.ConnectionTypeString, Label: "Subscription ID", Placeholder: "Azure subscription GUID", Required: true},
	{Name: "resource_id", Type: core.ConnectionTypeString, Label: "Resource ID", Placeholder: "Full ARM ID — /subscriptions/.../resourceGroups/.../providers/.../my-resource", Required: true},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Tags (JSON)", Placeholder: "{\"env\":\"prod\",\"owner\":\"ops\"}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "resource_id", Type: core.ConnectionTypeString, Label: "Resource ID"},
	{Name: "tags", Type: core.ConnectionTypeObject, Label: "Applied Tags"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := compute.GetAuth(inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	resourceID, err := compute.RequiredString("resource_id", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	tagMap, err := compute.TagMap("tags", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	if len(tagMap) == 0 {
		return compute.ErrorResult("at least one tag is required"), nil
	}

	client, err := auth.TagsClient()
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	ctx := compute.Context()
	_, err = client.UpdateAtScope(ctx, resourceID, armresources.TagsPatchResource{
		Operation:  to.Ptr(armresources.TagsPatchOperationMerge),
		Properties: &armresources.Tags{Tags: tagMap},
	}, nil)
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}

	applied := map[string]string{}
	for k, v := range tagMap {
		applied[k] = compute.Str(v)
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Set %d tag(s) on the resource", len(applied)),
		"resource_id": resourceID,
		"tags":        applied,
		"success":     true,
	}, nil
}
