// Package oracle_filestorage_outbound_connector_delete deletes an FSS outbound connector by OCID.
package oracle_filestorage_outbound_connector_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	fss "flomation.app/automate/executor/actions/oracle/filestorage"

	filestorage "github.com/oracle/oci-go-sdk/v65/filestorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI File Storage: Delete Outbound Connector"
	Description  = "Permanently delete an Oracle Cloud File Storage outbound connector (Kerberos/LDAP endpoint) by OCID. It moves to DELETING then DELETED; poll Get Outbound Connector (it 404s once gone)."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+server"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the outbound-connector picker)"},
	{Name: "outbound_connector_ocid", Type: core.ConnectionTypeString, Label: "Outbound Connector OCID", Placeholder: "ocid1.outboundconnector.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Outbound Connector OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := fss.ResourceClient(inputs, "outbound_connector_ocid")
	if errResult != nil {
		return errResult, nil
	}
	if _, err := client.DeleteOutboundConnector(fss.Context(), filestorage.DeleteOutboundConnectorRequest{OutboundConnectorId: &id}); err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}
	return fss.Result(fmt.Sprintf("Deleted outbound connector %s", id), map[string]interface{}{"id": id}), nil
}
