// Package oracle_waf_network_address_list_delete deletes a WAF NetworkAddressList by its OCID.
// Asynchronous: the delete returns a work-request id you can poll for completion.
package oracle_waf_network_address_list_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	wf "flomation.app/automate/executor/actions/oracle/waf"

	"github.com/oracle/oci-go-sdk/v65/waf"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI WAF: Delete Network Address List"
	Description  = "Delete a WAF network address list by its OCID. Returns a work-request id — the removal is asynchronous."
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
	{Name: "network_address_list_ocid", Type: core.ConnectionTypeString, Label: "Network Address List OCID", Placeholder: "ocid1.wafnetworkaddresslist.oc1..aaaa… of the list to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Network Address List OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := wf.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	listID, err := wf.RequiredString("network_address_list_ocid", inputs)
	if err != nil {
		return wf.ErrorResult(err.Error()), nil
	}

	resp, err := client.DeleteNetworkAddressList(wf.Context(), waf.DeleteNetworkAddressListRequest{NetworkAddressListId: &listID})
	if err != nil {
		return wf.ErrorResult(auth.OCIError(err)), nil
	}
	return wf.Result(fmt.Sprintf("Deleting network address list %s", listID), map[string]interface{}{
		"id":              listID,
		"work_request_id": wf.Str(resp.OpcWorkRequestId),
	}), nil
}
