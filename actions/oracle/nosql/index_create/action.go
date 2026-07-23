// Package oracle_nosql_index_create creates a secondary index on a NoSQL Database table. An index
// is defined by a name plus an ordered set of key columns to index on. The create runs
// asynchronously and returns a work-request OCID — poll it until the index is ACTIVE before use.
package oracle_nosql_index_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	ns "flomation.app/automate/executor/actions/oracle/nosql"

	"github.com/oracle/oci-go-sdk/v65/nosql"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI NoSQL: Create Index"
	Description  = "Create a secondary index on a NoSQL table — give it a name and a comma-separated list of key columns. Runs asynchronously and returns a work-request OCID to track until ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+table"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (the table's compartment)", Required: true},
	{Name: "table_ocid_or_name", Type: core.ConnectionTypeString, Label: "Table OCID or Name", Placeholder: "A table OCID, or a table name within the compartment", Required: true},
	{Name: "index_name", Type: core.ConnectionTypeString, Label: "Index Name", Placeholder: "A name for the new index", Required: true},
	{Name: "columns", Type: core.ConnectionTypeString, Label: "Key Columns (CSV)", Placeholder: "Comma-separated column names, e.g. lastName,firstName", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Table OCID or Name"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
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
	table, err := ns.RequiredString("table_ocid_or_name", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	indexName, err := ns.RequiredString("index_name", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	columnsRaw, err := ns.RequiredString("columns", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}

	keys := make([]nosql.IndexKey, 0)
	for _, part := range strings.Split(columnsRaw, ",") {
		col := strings.TrimSpace(part)
		if col == "" {
			continue
		}
		c := col
		keys = append(keys, nosql.IndexKey{ColumnName: &c})
	}
	if len(keys) == 0 {
		return ns.ErrorResult("columns must list at least one column name"), nil
	}

	resp, err := client.CreateIndex(ns.Context(), nosql.CreateIndexRequest{
		TableNameOrId: &table,
		CreateIndexDetails: nosql.CreateIndexDetails{
			Name:          &indexName,
			Keys:          keys,
			CompartmentId: &compartment,
		},
	})
	if err != nil {
		return ns.ErrorResult(auth.OCIError(err)), nil
	}

	return ns.Result(fmt.Sprintf("Creating index %q on table %s — poll the work request until ACTIVE", indexName, table), map[string]interface{}{
		"id":              table,
		"work_request_id": ns.Str(resp.OpcWorkRequestId),
	}), nil
}
