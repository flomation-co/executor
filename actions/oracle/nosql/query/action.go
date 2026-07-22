// Package oracle_nosql_query runs a NoSQL SQL SELECT statement against a compartment's tables and
// returns the matching rows. Walks pagination up to a safe cap.
package oracle_nosql_query

import (
	"fmt"

	core "flomation.app/automate/executor"
	ns "flomation.app/automate/executor/actions/oracle/nosql"

	"github.com/oracle/oci-go-sdk/v65/nosql"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI NoSQL: Run Query"
	Description  = "Run a NoSQL SQL SELECT statement against the tables in a compartment and return the matching rows. Walks pagination up to a safe cap."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "statement", Type: core.ConnectionTypeText, Label: "SQL Statement", Placeholder: "SELECT * FROM my_table WHERE id = 1", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "rows", Type: core.ConnectionTypeObject, Label: "Rows"},
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
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	statement, err := ns.RequiredString("statement", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	req := nosql.QueryRequest{QueryDetails: nosql.QueryDetails{
		CompartmentId: &compartment,
		Statement:     &statement,
	}}
	var rows []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= ns.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.Query(ns.Context(), req)
		if err != nil {
			return ns.ErrorResult(auth.OCIError(err)), nil
		}
		rows = append(rows, resp.Items...)
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return ns.Result(fmt.Sprintf("Query returned %d row(s)", len(rows)), map[string]interface{}{
		"rows": rows, "count": fmt.Sprintf("%d", len(rows)), "truncated": truncated,
	}), nil
}
