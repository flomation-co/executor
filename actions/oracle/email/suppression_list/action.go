// Package oracle_email_suppression_list lists the addresses on a compartment's email suppression
// list, optionally filtered by exact email address or a creation-time window. Walks pagination up
// to a safe cap.
package oracle_email_suppression_list

import (
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	em "flomation.app/automate/executor/actions/oracle/email"

	ocicommon "github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/email"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Email: List Suppressions"
	Description  = "List the addresses on a compartment's email suppression list. Optionally filter by exact email address or a creation-time window, and sort. Walks pagination up to a safe cap."
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
	{Name: "email_address", Type: core.ConnectionTypeString, Label: "Email Address Filter", Placeholder: "Only the suppression for this exact address (optional)"},
	{Name: "time_created_greater_than_or_equal_to", Type: core.ConnectionTypeString, Label: "Created On/After", Placeholder: "RFC3339, e.g. 2026-07-01T00:00:00Z (optional)"},
	{Name: "time_created_less_than", Type: core.ConnectionTypeString, Label: "Created Before", Placeholder: "RFC3339, e.g. 2026-07-22T00:00:00Z (optional)"},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Results per page, 1–1000 (optional)"},
	{Name: "sort_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "Defaults to TIMECREATED", Options: []core.ConnectionOption{
		{Name: "Time Created", Value: "TIMECREATED"}, {Name: "Email Address", Value: "EMAILADDRESS"},
	}},
	{Name: "sort_order", Type: core.ConnectionTypeString, Label: "Sort Order", Placeholder: "Ascending or descending", Options: []core.ConnectionOption{
		{Name: "Ascending", Value: "ASC"}, {Name: "Descending", Value: "DESC"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "suppressions", Type: core.ConnectionTypeObject, Label: "Suppressions"},
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
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return em.ErrorResult(err.Error()), nil
	}

	req := email.ListSuppressionsRequest{CompartmentId: &compartment}
	if addr := em.OptionalString("email_address", inputs); addr != "" {
		req.EmailAddress = &addr
	}
	if v := em.OptionalString("time_created_greater_than_or_equal_to", inputs); v != "" {
		parsed, perr := time.Parse(time.RFC3339, v)
		if perr != nil {
			return em.ErrorResult("created on/after must be RFC3339, e.g. 2026-07-01T00:00:00Z"), nil
		}
		req.TimeCreatedGreaterThanOrEqualTo = &ocicommon.SDKTime{Time: parsed.UTC()}
	}
	if v := em.OptionalString("time_created_less_than", inputs); v != "" {
		parsed, perr := time.Parse(time.RFC3339, v)
		if perr != nil {
			return em.ErrorResult("created before must be RFC3339, e.g. 2026-07-22T00:00:00Z"), nil
		}
		req.TimeCreatedLessThan = &ocicommon.SDKTime{Time: parsed.UTC()}
	}
	if n, ok, ierr := em.OptionalInt("limit", inputs); ierr != nil {
		return em.ErrorResult(ierr.Error()), nil
	} else if ok {
		req.Limit = &n
	}
	if v := em.OptionalString("sort_by", inputs); v != "" {
		req.SortBy = email.ListSuppressionsSortByEnum(v)
	}
	if v := em.OptionalString("sort_order", inputs); v != "" {
		req.SortOrder = email.ListSuppressionsSortOrderEnum(v)
	}

	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= em.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListSuppressions(em.Context(), req)
		if err != nil {
			return em.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, em.SummariseSuppressionSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}

	return em.Result(fmt.Sprintf("Found %d suppression(s)", len(out)), map[string]interface{}{
		"suppressions": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
