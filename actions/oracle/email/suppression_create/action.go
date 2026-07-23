// Package oracle_email_suppression_create adds a single recipient address to the customer-level
// suppression list, so the Email Delivery service will refuse to send to it. Suppressions are a
// tenancy-wide (customer-level) list, so the compartment must be the tenancy OCID.
package oracle_email_suppression_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	em "flomation.app/automate/executor/actions/oracle/email"

	"github.com/oracle/oci-go-sdk/v65/email"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Email: Create Suppression"
	Description  = "Add a recipient email address to the suppression list so Email Delivery will not send to it. Suppressions are customer-level, so the compartment must be the tenancy OCID. The reason is set by OCI (MANUAL for hand-added addresses) and returned on the result."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+envelope"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "email_address", Type: core.ConnectionTypeString, Label: "Email Address", Placeholder: "The recipient address to suppress, e.g. bounced@example.com", Required: true},
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
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return em.ErrorResult(err.Error()), nil
	}
	address, err := em.RequiredString("email_address", inputs)
	if err != nil {
		return em.ErrorResult(err.Error()), nil
	}

	details := email.CreateSuppressionDetails{
		CompartmentId: &compartment,
		EmailAddress:  &address,
	}

	resp, err := client.CreateSuppression(em.Context(), email.CreateSuppressionRequest{CreateSuppressionDetails: details})
	if err != nil {
		return em.ErrorResult(auth.OCIError(err)), nil
	}
	suppression := em.SummariseSuppression(&resp.Suppression)
	return em.Result(fmt.Sprintf("Suppressed %q (reason %s)", suppression["email_address"], suppression["reason"]), map[string]interface{}{
		"suppression": suppression,
		"id":          suppression["id"],
	}), nil
}
