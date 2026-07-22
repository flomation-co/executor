// Package oracle_nosql_index_list lists the secondary indexes of a NoSQL Database table,
// identified by table name or OCID. Walks pagination up to a safe cap.
package oracle_nosql_index_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	ns "flomation.app/automate/executor/actions/oracle/nosql"

	"github.com/oracle/oci-go-sdk/v65/nosql"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI NoSQL: List Indexes"
	Description  = "List the secondary indexes of a NoSQL Database table, identified by table name or OCID. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+table"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… — needed to interpret a table name (optional)"},
	{Name: "table_ocid_or_name", Type: core.ConnectionTypeString, Label: "Table Name or OCID", Placeholder: "A table name within the compartment, or a table OCID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "indexes", Type: core.ConnectionTypeObject, Label: "Indexes"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := ns.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	table, err := ns.RequiredString("table_ocid_or_name", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	req := nosql.ListIndexesRequest{TableNameOrId: &table}
	if auth.CompartmentOCID != "" {
		c := auth.CompartmentOCID
		req.CompartmentId = &c
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= ns.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListIndexes(ns.Context(), req)
		if err != nil {
			return ns.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, ns.SummariseIndexSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return ns.Result(fmt.Sprintf("Found %d index(es)", len(out)), map[string]interface{}{
		"indexes": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
