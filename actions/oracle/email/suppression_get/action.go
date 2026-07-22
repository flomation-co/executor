// Package oracle_email_suppression_get fetches a single Email Delivery suppression-list entry by
// its OCID, returning the suppressed address, the reason it was added and when it was last suppressed.
package oracle_email_suppression_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	em "flomation.app/automate/executor/actions/oracle/email"

	"github.com/oracle/oci-go-sdk/v65/email"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Email: Get Suppression"
	Description  = "Fetch a single Email Delivery suppression-list entry by its OCID — the address, reason and suppression timestamps."
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
	{Name: "suppression_ocid", Type: core.ConnectionTypeString, Label: "Suppression OCID", Placeholder: "ocid1.emailsuppression.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "suppression", Type: core.ConnectionTypeObject, Label: "Suppression"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Suppression OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := em.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	suppressionID, err := em.RequiredString("suppression_ocid", inputs)
	if err != nil {
		return em.ErrorResult(err.Error()), nil
	}

	resp, err := client.GetSuppression(em.Context(), email.GetSuppressionRequest{SuppressionId: &suppressionID})
	if err != nil {
		return em.ErrorResult(auth.OCIError(err)), nil
	}
	suppression := em.SummariseSuppression(&resp.Suppression)
	return em.Result(fmt.Sprintf("Suppression for %q (%s)", suppression["email_address"], suppression["reason"]), map[string]interface{}{
		"suppression": suppression,
		"id":          suppression["id"],
	}), nil
}
