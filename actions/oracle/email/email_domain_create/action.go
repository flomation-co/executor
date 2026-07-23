// Package oracle_email_email_domain_create creates an email (sending) domain — the DNS domain that
// approved senders send from and that carries the DKIM signing keys. Asynchronous: the request
// returns the new domain plus a work-request id; poll Get Email Domain until it is ACTIVE.
package oracle_email_email_domain_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	em "flomation.app/automate/executor/actions/oracle/email"

	"github.com/oracle/oci-go-sdk/v65/email"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Email: Create Email Domain"
	Description  = "Create an email sending domain in a compartment. Returns the domain plus a work-request id — poll Get Email Domain until ACTIVE."
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Domain Name", Placeholder: "The DNS domain to send from, e.g. mail.example.com", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "email_domain", Type: core.ConnectionTypeObject, Label: "Email Domain"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Email Domain OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
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
	name, err := em.RequiredString("name", inputs)
	if err != nil {
		return em.ErrorResult(err.Error()), nil
	}

	details := email.CreateEmailDomainDetails{Name: &name, CompartmentId: &compartment}

	resp, err := client.CreateEmailDomain(em.Context(), email.CreateEmailDomainRequest{CreateEmailDomainDetails: details})
	if err != nil {
		return em.ErrorResult(auth.OCIError(err)), nil
	}

	domain := em.SummariseEmailDomain(&resp.EmailDomain)
	return em.Result(fmt.Sprintf("Creating email domain %q — poll Get Email Domain until ACTIVE", name), map[string]interface{}{
		"email_domain":    domain,
		"id":              em.Str(resp.Id),
		"lifecycle_state": string(resp.LifecycleState),
		"work_request_id": em.Str(resp.OpcWorkRequestId),
	}), nil
}
