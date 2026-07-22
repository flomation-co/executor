// Package oracle_nosql_row_update puts (inserts or replaces) a single row in a NoSQL Database
// table from a JSON object of column values, returning the new row version.
package oracle_nosql_row_update

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	ns "flomation.app/automate/executor/actions/oracle/nosql"

	"github.com/oracle/oci-go-sdk/v65/nosql"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI NoSQL: Update Row"
	Description  = "Put (insert or replace) a single row in a NoSQL Database table from a JSON object of column values, returning the new row version."
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
	{Name: "table_ocid_or_name", Type: core.ConnectionTypeString, Label: "Table Name or OCID", Placeholder: "A table name within the compartment, or a table OCID", Required: true},
	{Name: "value_json", Type: core.ConnectionTypeText, Label: "Row Value (JSON)", Placeholder: "{\"id\":1,\"name\":\"Ada\"} — a JSON object mapping every primary-key (and any) column to its value", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "version", Type: core.ConnectionTypeString, Label: "Row Version"},
	{Name: "existing_version", Type: core.ConnectionTypeString, Label: "Existing Version"},
	{Name: "generated_value", Type: core.ConnectionTypeString, Label: "Generated Identity Value"},
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
	rawValue, err := ns.RequiredString("value_json", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	var value map[string]interface{}
	if err := json.Unmarshal([]byte(rawValue), &value); err != nil {
		return ns.ErrorResult(fmt.Sprintf("row value must be a JSON object of column values, e.g. {\"id\":1,\"name\":\"Ada\"}: %s", err.Error())), nil
	}
	if len(value) == 0 {
		return ns.ErrorResult("row value must contain at least one column"), nil
	}

	details := nosql.UpdateRowDetails{Value: value, CompartmentId: &compartment}
	resp, err := client.UpdateRow(ns.Context(), nosql.UpdateRowRequest{
		TableNameOrId:    &table,
		UpdateRowDetails: details,
	})
	if err != nil {
		return ns.ErrorResult(auth.OCIError(err)), nil
	}

	version := ns.Str(resp.Version)
	existingVersion := ns.Str(resp.ExistingVersion)
	generated := ns.Str(resp.GeneratedValue)

	msg := fmt.Sprintf("Wrote row to table %q", table)
	if strings.TrimSpace(version) == "" && strings.TrimSpace(existingVersion) != "" {
		msg = fmt.Sprintf("Row not written to table %q — a condition (option) was not met; the existing row remains", table)
	}
	return ns.Result(msg, map[string]interface{}{
		"version":          version,
		"existing_version": existingVersion,
		"generated_value":  generated,
	}), nil
}
