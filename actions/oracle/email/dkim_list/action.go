// Package oracle_email_dkim_list lists the DKIM signing keys belonging to an email domain,
// optionally filtered by exact name or lifecycle state. Walks pagination up to a safe cap.
package oracle_email_dkim_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	em "flomation.app/automate/executor/actions/oracle/email"

	"github.com/oracle/oci-go-sdk/v65/email"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Email: List DKIMs"
	Description  = "List the DKIM signing keys belonging to an email domain. Optionally filter by exact name or lifecycle state. Walks pagination up to a safe cap."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "email_domain_ocid", Type: core.ConnectionTypeString, Label: "Email Domain OCID", Placeholder: "ocid1.emaildomain.oc1..aaaa… — the domain whose DKIMs to list", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name Filter", Placeholder: "Only DKIMs with this exact name (optional)"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State", Placeholder: "Filter by state (optional)", Options: []core.ConnectionOption{
		{Name: "Active", Value: "ACTIVE"},
		{Name: "Creating", Value: "CREATING"},
		{Name: "Deleting", Value: "DELETING"},
		{Name: "Deleted", Value: "DELETED"},
		{Name: "Failed", Value: "FAILED"},
		{Name: "Inactive", Value: "INACTIVE"},
		{Name: "Needs Attention", Value: "NEEDS_ATTENTION"},
		{Name: "Updating", Value: "UPDATING"},
	}},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Max results per page, 1–1000 (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "dkims", Type: core.ConnectionTypeObject, Label: "DKIMs"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := em.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	emailDomainID, err := em.RequiredString("email_domain_ocid", inputs)
	if err != nil {
		return em.ErrorResult(err.Error()), nil
	}
	req := email.ListDkimsRequest{EmailDomainId: &emailDomainID}
	if name := em.OptionalString("name", inputs); name != "" {
		req.Name = &name
	}
	if state := em.OptionalString("lifecycle_state", inputs); state != "" {
		req.LifecycleState = email.DkimLifecycleStateEnum(state)
	}
	if limit, ok, err := em.OptionalInt("limit", inputs); err != nil {
		return em.ErrorResult(err.Error()), nil
	} else if ok {
		req.Limit = &limit
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= em.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListDkims(em.Context(), req)
		if err != nil {
			return em.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, em.SummariseDkimSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return em.Result(fmt.Sprintf("Found %d DKIM(s)", len(out)), map[string]interface{}{
		"dkims": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
