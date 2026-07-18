// Package azure_compute_nsg_add_inbound_rule adds (or updates) an inbound
// security rule on a Network Security Group — the Azure equivalent of EC2's
// authorize-security-group-ingress. The SDK create is a long-running operation
// Execute polls to completion.
package azure_compute_nsg_add_inbound_rule

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
	Name         = "Azure NSG: Add Inbound Rule"
	Description  = "Add or update an inbound rule on a Network Security Group (protocol, ports, source/destination, allow or deny)."
	Website      = "https://www.flomation.co"
	Icon         = "azure+plus"
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
	{Name: "rule_name", Type: core.ConnectionTypeString, Label: "Rule Name", Placeholder: "allow-ssh", Required: true},
	{Name: "priority", Type: core.ConnectionTypeInteger, Label: "Priority", Placeholder: "100–4096; lower wins", Required: true},
	{Name: "protocol", Type: core.ConnectionTypeString, Label: "Protocol", Required: true, Options: []core.ConnectionOption{{Name: "TCP", Value: "Tcp"}, {Name: "UDP", Value: "Udp"}, {Name: "Any", Value: "*"}}},
	{Name: "destination_port_range", Type: core.ConnectionTypeString, Label: "Destination Port(s)", Placeholder: "22, or 80, or a range 8000-8100, or * for any", Required: true},
	{Name: "access", Type: core.ConnectionTypeString, Label: "Access", Options: []core.ConnectionOption{{Name: "Allow", Value: "Allow"}, {Name: "Deny", Value: "Deny"}}},
	{Name: "source_address_prefix", Type: core.ConnectionTypeString, Label: "Source Address", Placeholder: "* (any), a CIDR like 10.0.0.0/24, or a tag like Internet — default *"},
	{Name: "source_port_range", Type: core.ConnectionTypeString, Label: "Source Port(s)", Placeholder: "* (any) — default *"},
	{Name: "destination_address_prefix", Type: core.ConnectionTypeString, Label: "Destination Address", Placeholder: "* (any), a CIDR, or a tag like VirtualNetwork — default *"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Rule Name"},
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
	nsgName, err := compute.RequiredString("nsg_name", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	ruleName, err := compute.RequiredString("rule_name", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	priority, err := compute.RequiredInt("priority", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	protocol, err := compute.RequiredString("protocol", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	destPort, err := compute.RequiredString("destination_port_range", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	access := compute.OptionalString("access", inputs)
	if access == "" {
		access = "Allow"
	}
	sourceAddr := compute.OptionalString("source_address_prefix", inputs)
	if sourceAddr == "" {
		sourceAddr = "*"
	}
	sourcePort := compute.OptionalString("source_port_range", inputs)
	if sourcePort == "" {
		sourcePort = "*"
	}
	destAddr := compute.OptionalString("destination_address_prefix", inputs)
	if destAddr == "" {
		destAddr = "*"
	}

	rule := armnetwork.SecurityRule{
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Priority:                 to.Ptr(int32(priority)),
			Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
			Access:                   to.Ptr(armnetwork.SecurityRuleAccess(access)),
			Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocol(protocol)),
			SourceAddressPrefix:      to.Ptr(sourceAddr),
			SourcePortRange:          to.Ptr(sourcePort),
			DestinationAddressPrefix: to.Ptr(destAddr),
			DestinationPortRange:     to.Ptr(destPort),
		},
	}

	client, err := auth.SecurityRulesClient()
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	ctx := compute.Context()
	poller, err := client.BeginCreateOrUpdate(ctx, rg, nsgName, ruleName, rule, nil)
	if err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return compute.ErrorResult(auth.AzureError(err)), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("%s inbound %s to %s on %q (priority %d)", access, protocol, destPort, nsgName, priority),
		"name":        ruleName,
		"success":     true,
	}, nil
}
