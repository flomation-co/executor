// Package oracle_email_email_domain_get fetches a single OCI Email Delivery sending domain by its
// OCID, returning its name, verification status, active DKIM key and lifecycle state.
package oracle_email_email_domain_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	em "flomation.app/automate/executor/actions/oracle/email"

	"github.com/oracle/oci-go-sdk/v65/email"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Email: Get Email Domain"
	Description  = "Fetch a single Email Delivery sending domain by its OCID — its name, verification status, active DKIM key and lifecycle state."
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
	{Name: "email_domain_ocid", Type: core.ConnectionTypeString, Label: "Email Domain OCID", Placeholder: "ocid1.emaildomain.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "email_domain", Type: core.ConnectionTypeObject, Label: "Email Domain"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Email Domain OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := em.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	domainID, err := em.RequiredString("email_domain_ocid", inputs)
	if err != nil {
		return em.ErrorResult(err.Error()), nil
	}

	resp, err := client.GetEmailDomain(em.Context(), email.GetEmailDomainRequest{EmailDomainId: &domainID})
	if err != nil {
		return em.ErrorResult(auth.OCIError(err)), nil
	}
	domain := em.SummariseEmailDomain(&resp.EmailDomain)
	return em.Result(fmt.Sprintf("Email domain %q (%s)", domain["name"], domain["lifecycle_state"]), map[string]interface{}{
		"email_domain":    domain,
		"id":              domain["id"],
		"lifecycle_state": domain["lifecycle_state"],
	}), nil
}
