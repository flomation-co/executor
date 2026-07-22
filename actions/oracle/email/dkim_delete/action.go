// Package oracle_email_dkim_delete deletes an Email Delivery DKIM signing key by its OCID; the
// service returns a work-request OCID that tracks the asynchronous teardown.
package oracle_email_dkim_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	em "flomation.app/automate/executor/actions/oracle/email"

	"github.com/oracle/oci-go-sdk/v65/email"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Email: Delete DKIM"
	Description  = "Delete an Email Delivery DKIM signing key by its OCID, returning the work-request that tracks the teardown."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+envelope"
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
	{Name: "dkim_ocid", Type: core.ConnectionTypeString, Label: "DKIM OCID", Placeholder: "ocid1.dkim.oc1..aaaa… of the DKIM to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "DKIM OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := em.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	dkimID, err := em.RequiredString("dkim_ocid", inputs)
	if err != nil {
		return em.ErrorResult(err.Error()), nil
	}

	resp, err := client.DeleteDkim(em.Context(), email.DeleteDkimRequest{DkimId: &dkimID})
	if err != nil {
		return em.ErrorResult(auth.OCIError(err)), nil
	}
	return em.Result(fmt.Sprintf("Deleted DKIM %s", dkimID), map[string]interface{}{
		"id":              dkimID,
		"work_request_id": em.Str(resp.OpcWorkRequestId),
	}), nil
}
