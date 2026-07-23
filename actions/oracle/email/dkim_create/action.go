// Package oracle_email_dkim_create creates a DKIM signing key for an email domain. OCI generates the
// RSA key pair and returns a CNAME record you publish in DNS so recipients can verify your mail.
// Asynchronous: the DKIM comes back CREATING with a work-request id — poll Get DKIM until ACTIVE.
package oracle_email_dkim_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	em "flomation.app/automate/executor/actions/oracle/email"

	"github.com/oracle/oci-go-sdk/v65/email"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Email: Create DKIM"
	Description  = "Create a DKIM signing key for an email domain. Optionally set the selector name — leave it blank to let OCI generate one. Returns a work-request id; poll Get DKIM until ACTIVE, then publish the CNAME record in DNS."
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
	{Name: "email_domain_ocid", Type: core.ConnectionTypeString, Label: "Email Domain OCID", Placeholder: "ocid1.emaildomain.oc1..aaaa…", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Selector Name", Placeholder: "DKIM selector, e.g. mydomain-lhr-20260722 (optional — OCI generates one if blank)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "dkim", Type: core.ConnectionTypeObject, Label: "DKIM"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "DKIM OCID"},
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
	domainID, err := em.RequiredString("email_domain_ocid", inputs)
	if err != nil {
		return em.ErrorResult(err.Error()), nil
	}

	details := email.CreateDkimDetails{EmailDomainId: &domainID}
	if name := em.OptionalString("name", inputs); name != "" {
		details.Name = &name
	}

	resp, err := client.CreateDkim(em.Context(), email.CreateDkimRequest{CreateDkimDetails: details})
	if err != nil {
		return em.ErrorResult(auth.OCIError(err)), nil
	}

	out := map[string]interface{}{
		"dkim":            em.SummariseDkim(&resp.Dkim),
		"id":              em.Str(resp.Id),
		"lifecycle_state": string(resp.LifecycleState),
		"work_request_id": em.Str(resp.OpcWorkRequestId),
	}
	return em.Result(fmt.Sprintf("Creating DKIM %q for email domain — poll Get DKIM until ACTIVE, then publish the CNAME record", em.Str(resp.Name)), out), nil
}
