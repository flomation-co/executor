// Package oracle_datacatalog_glossary_list lists the business glossaries in a Data Catalog,
// optionally filtered by exact display name. Glossaries are child resources identified by a string
// Key and scoped by the catalog OCID. Walks pagination up to a safe cap.
package oracle_datacatalog_glossary_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	dc "flomation.app/automate/executor/actions/oracle/datacatalog"

	"github.com/oracle/oci-go-sdk/v65/datacatalog"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Catalog: List Glossaries"
	Description  = "List the business glossaries in a Data Catalog. Optionally filter by exact display name or cap the number of results. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+book"
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
	{Name: "catalog_ocid", Type: core.ConnectionTypeString, Label: "Catalog OCID", Placeholder: "ocid1.datacatalog.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name Filter", Placeholder: "Only glossaries with this exact name (optional)"},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit", Placeholder: "Max items to request per page (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "glossaries", Type: core.ConnectionTypeObject, Label: "Glossaries"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := dc.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	catalogID, err := dc.RequiredString("catalog_ocid", inputs)
	if err != nil {
		return dc.ErrorResult(err.Error()), nil
	}
	req := datacatalog.ListGlossariesRequest{CatalogId: &catalogID}
	if name := dc.OptionalString("display_name", inputs); name != "" {
		req.DisplayName = &name
	}
	if n, ok, err := dc.OptionalInt("limit", inputs); err != nil {
		return dc.ErrorResult(err.Error()), nil
	} else if ok {
		req.Limit = &n
	}

	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= dc.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListGlossaries(dc.Context(), req)
		if err != nil {
			return dc.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, dc.SummariseGlossarySummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return dc.Result(fmt.Sprintf("Found %d glossary(ies)", len(out)), map[string]interface{}{
		"glossaries": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
