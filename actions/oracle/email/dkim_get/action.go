// Package oracle_email_dkim_get fetches a single Email Delivery DKIM signing key by OCID,
// returning its DNS subdomain, CNAME record value, key length and lifecycle state.
package oracle_email_dkim_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	em "flomation.app/automate/executor/actions/oracle/email"

	"github.com/oracle/oci-go-sdk/v65/email"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Email: Get DKIM"
	Description  = "Fetch a single Email Delivery DKIM signing key by its OCID — DNS subdomain, CNAME record value and lifecycle state."
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
	{Name: "dkim_ocid", Type: core.ConnectionTypeString, Label: "DKIM OCID", Placeholder: "ocid1.dkim.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "dkim", Type: core.ConnectionTypeObject, Label: "DKIM"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "DKIM OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
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

	resp, err := client.GetDkim(em.Context(), email.GetDkimRequest{DkimId: &dkimID})
	if err != nil {
		return em.ErrorResult(auth.OCIError(err)), nil
	}
	dkim := em.SummariseDkim(&resp.Dkim)
	return em.Result(fmt.Sprintf("DKIM %q (%s)", dkim["name"], dkim["lifecycle_state"]), map[string]interface{}{
		"dkim": dkim, "id": dkim["id"], "lifecycle_state": dkim["lifecycle_state"],
	}), nil
}
