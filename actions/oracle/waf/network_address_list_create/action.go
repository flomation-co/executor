// Package oracle_waf_network_address_list_create creates a WAF NetworkAddressList of the ADDRESSES
// kind — a reusable set of IP address prefixes (CIDR) that policies reference for allow/deny rules.
// Asynchronous: the create returns a work-request id you can poll for completion.
package oracle_waf_network_address_list_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	wf "flomation.app/automate/executor/actions/oracle/waf"

	"github.com/oracle/oci-go-sdk/v65/waf"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI WAF: Create Network Address List"
	Description  = "Create a WAF network address list from a comma-separated set of IP address prefixes (CIDR). Returns a work-request id — the creation is asynchronous."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+shield-virus"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the network address list (optional)"},
	{Name: "addresses", Type: core.ConnectionTypeString, Label: "Addresses (CIDR, comma-separated)", Placeholder: "e.g. 192.0.2.0/24, 203.0.113.5/32, ::/0", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := wf.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return wf.ErrorResult(err.Error()), nil
	}
	raw, err := wf.RequiredString("addresses", inputs)
	if err != nil {
		return wf.ErrorResult(err.Error()), nil
	}
	var addresses []string
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			addresses = append(addresses, p)
		}
	}
	if len(addresses) == 0 {
		return wf.ErrorResult("addresses is required — provide at least one IP address prefix (CIDR)"), nil
	}

	details := waf.CreateNetworkAddressListAddressesDetails{
		CompartmentId: &compartment,
		Addresses:     addresses,
	}
	if name := wf.OptionalString("display_name", inputs); name != "" {
		details.DisplayName = &name
	}

	resp, err := client.CreateNetworkAddressList(wf.Context(), waf.CreateNetworkAddressListRequest{
		CreateNetworkAddressListDetails: details,
	})
	if err != nil {
		return wf.ErrorResult(auth.OCIError(err)), nil
	}

	label := wf.Str(details.DisplayName)
	if label == "" {
		label = "network address list"
	}
	return wf.Result(fmt.Sprintf("Creating WAF %s with %d address(es) — poll for the work request to complete", label, len(addresses)), map[string]interface{}{
		"work_request_id": wf.Str(resp.OpcWorkRequestId),
	}), nil
}
