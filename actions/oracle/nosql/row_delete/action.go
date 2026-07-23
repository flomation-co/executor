// Package oracle_nosql_row_delete deletes a single row from a NoSQL Database table by its primary
// key. The table may be named or given by OCID; when named, supply the compartment for context.
package oracle_nosql_row_delete

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
	Name         = "OCI NoSQL: Delete Row"
	Description  = "Delete a single row from a NoSQL Database table by its primary key."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… — needed when the table is given by name (optional)"},
	{Name: "table_ocid_or_name", Type: core.ConnectionTypeString, Label: "Table Name or OCID", Placeholder: "A table name within the compartment, or a table OCID", Required: true},
	{Name: "key", Type: core.ConnectionTypeString, Label: "Primary Key", Placeholder: "Comma-separated column:value pairs, e.g. id:42,region:emea", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
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
	rawKey, err := ns.RequiredString("key", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	var key []string
	for _, part := range strings.Split(rawKey, ",") {
		if p := strings.TrimSpace(part); p != "" {
			key = append(key, p)
		}
	}
	if len(key) == 0 {
		return ns.ErrorResult("key must contain at least one column:value pair, e.g. id:42"), nil
	}

	req := nosql.DeleteRowRequest{TableNameOrId: &table, Key: key}
	if auth.CompartmentOCID != "" {
		req.CompartmentId = &auth.CompartmentOCID
	}

	resp, err := client.DeleteRow(ns.Context(), req)
	if err != nil {
		return ns.ErrorResult(auth.OCIError(err)), nil
	}
	deleted := resp.IsSuccess != nil && *resp.IsSuccess
	var msg string
	if deleted {
		msg = fmt.Sprintf("Deleted row %s from table %s", rawKey, table)
	} else {
		msg = fmt.Sprintf("No matching row to delete for key %s in table %s", rawKey, table)
	}
	return ns.Result(msg, nil), nil
}
