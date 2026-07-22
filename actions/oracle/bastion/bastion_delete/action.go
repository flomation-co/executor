// Package oracle_bastion_bastion_delete deletes an OCI Bastion by its OCID. Deletion is
// asynchronous — the call returns a work-request OCID you can poll for completion.
package oracle_bastion_bastion_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	bas "flomation.app/automate/executor/actions/oracle/bastion"

	"github.com/oracle/oci-go-sdk/v65/bastion"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Bastion: Delete Bastion"
	Description  = "Delete an OCI Bastion by its OCID — returns a work-request OCID to poll for completion."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+terminal"
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
	{Name: "bastion_ocid", Type: core.ConnectionTypeString, Label: "Bastion OCID", Placeholder: "ocid1.bastion.oc1..aaaa… of the bastion to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Bastion OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := bas.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	bastionID, err := bas.RequiredString("bastion_ocid", inputs)
	if err != nil {
		return bas.ErrorResult(err.Error()), nil
	}

	resp, err := client.DeleteBastion(bas.Context(), bastion.DeleteBastionRequest{BastionId: &bastionID})
	if err != nil {
		return bas.ErrorResult(auth.OCIError(err)), nil
	}
	return bas.Result(fmt.Sprintf("Deletion of bastion %s accepted", bastionID), map[string]interface{}{
		"id":              bastionID,
		"work_request_id": bas.Str(resp.OpcWorkRequestId),
	}), nil
}
