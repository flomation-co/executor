// Package oracle_nosql_table_change_compartment moves a NoSQL Database table from one compartment
// to another. The table keeps its OCID; only its compartment placement (for access control and
// billing) changes. The move runs asynchronously and returns a work-request OCID to track it.
package oracle_nosql_table_change_compartment

import (
	"fmt"

	core "flomation.app/automate/executor"
	ns "flomation.app/automate/executor/actions/oracle/nosql"

	"github.com/oracle/oci-go-sdk/v65/nosql"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI NoSQL: Change Table Compartment"
	Description  = "Move a NoSQL table into a different compartment — the table keeps its OCID, only its compartment placement changes."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (the table's current compartment)", Required: true},
	{Name: "table_ocid_or_name", Type: core.ConnectionTypeString, Label: "Table OCID or Name", Placeholder: "A table OCID, or a table name within the current compartment", Required: true},
	{Name: "destination_compartment_ocid", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (where to move the table)", Required: true},
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
	from, err := auth.RequiredCompartment()
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	table, err := ns.RequiredString("table_ocid_or_name", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	destination, err := ns.RequiredString("destination_compartment_ocid", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}

	resp, err := client.ChangeTableCompartment(ns.Context(), nosql.ChangeTableCompartmentRequest{
		TableNameOrId: &table,
		ChangeTableCompartmentDetails: nosql.ChangeTableCompartmentDetails{
			FromCompartmentId: &from,
			ToCompartmentId:   &destination,
		},
	})
	if err != nil {
		return ns.ErrorResult(auth.OCIError(err)), nil
	}

	return ns.Result(fmt.Sprintf("Moving table %s into compartment %s", table, destination), map[string]interface{}{
		"id":              table,
		"work_request_id": ns.Str(resp.OpcWorkRequestId),
	}), nil
}
