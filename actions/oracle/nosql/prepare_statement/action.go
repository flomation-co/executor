// Package oracle_nosql_prepare_statement compiles a NoSQL SQL statement into a reusable prepared
// statement, returning the base64/hex-encoded compiled form plus its query execution plan.
package oracle_nosql_prepare_statement

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"

	core "flomation.app/automate/executor"
	ns "flomation.app/automate/executor/actions/oracle/nosql"

	"github.com/oracle/oci-go-sdk/v65/nosql"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI NoSQL: Prepare Statement"
	Description  = "Compile a NoSQL SQL statement into a reusable prepared statement and return its query execution plan."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "statement", Type: core.ConnectionTypeText, Label: "SQL Statement", Placeholder: "A NoSQL SQL statement, e.g. SELECT * FROM my_table WHERE id = $id", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "prepared", Type: core.ConnectionTypeObject, Label: "Prepared Statement"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := ns.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartmentID, err := auth.RequiredCompartment()
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	statement, err := ns.RequiredString("statement", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}

	getPlan := true
	resp, err := client.PrepareStatement(ns.Context(), nosql.PrepareStatementRequest{
		CompartmentId:  &compartmentID,
		Statement:      &statement,
		IsGetQueryPlan: &getPlan,
	})
	if err != nil {
		return ns.ErrorResult(auth.OCIError(err)), nil
	}

	prepared := map[string]interface{}{
		"statement_base64": ns.Str(resp.Statement),
	}
	if resp.Statement != nil {
		if raw, decErr := base64.StdEncoding.DecodeString(*resp.Statement); decErr == nil {
			prepared["statement_hex"] = hex.EncodeToString(raw)
			prepared["statement_length_bytes"] = len(raw)
		}
	}
	if resp.QueryPlan != nil {
		prepared["query_plan"] = *resp.QueryPlan
	}
	if resp.Usage != nil {
		prepared["usage"] = map[string]interface{}{
			"read_units_consumed":  ns.IntOrNil(resp.Usage.ReadUnitsConsumed),
			"write_units_consumed": ns.IntOrNil(resp.Usage.WriteUnitsConsumed),
		}
	}

	summary := "Compiled the SQL statement into a prepared statement"
	if n, ok := prepared["statement_length_bytes"].(int); ok {
		summary = fmt.Sprintf("Compiled the SQL statement into a %d-byte prepared statement", n)
	}
	return ns.Result(summary, map[string]interface{}{
		"prepared": prepared,
	}), nil
}
