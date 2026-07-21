// Package oracle_networking_security_list_delete deletes a security list by OCID.
// The security list must not be associated with any subnet, and the VCN's default
// security list cannot be deleted.
package oracle_networking_security_list_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	net "flomation.app/automate/executor/actions/oracle/networking"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Networking: Delete Security List"
	Description  = "Delete an Oracle Cloud security list by OCID."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+shield-halved"
	Date         = "21/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "security_list_ocid", Type: core.ConnectionTypeString, Label: "Security List OCID", Placeholder: "ocid1.securitylist.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := net.NetworkResourceClient(inputs, "security_list_ocid")
	if errResult != nil {
		return errResult, nil
	}
	_, err := client.DeleteSecurityList(net.Context(), ocicore.DeleteSecurityListRequest{SecurityListId: &id})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Delete requested for security list %s", id),
		"success":     true,
	}, nil
}
