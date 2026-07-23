// Package oracle_nosql_table_update applies a partial alteration to a NoSQL Database table: change
// its throughput/storage limits and/or run an ALTER TABLE DDL statement. Asynchronous — the call
// returns a work-request id; poll Get Table until the table is back in an ACTIVE/updated state.
package oracle_nosql_table_update

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
	Name         = "OCI NoSQL: Update Table"
	Description  = "Partially update a NoSQL Database table — change its throughput/storage limits and/or run an ALTER TABLE DDL statement. Asynchronous: returns a work-request id, so poll Get Table until the change is applied."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (required to resolve a table by name)", Required: true},
	{Name: "table_ocid_or_name", Type: core.ConnectionTypeString, Label: "Table OCID or Name", Placeholder: "ocid1.nosqltable.oc1..aaaa… or a table name — the table to update", Required: true},
	{Name: "max_read_units", Type: core.ConnectionTypeString, Label: "Max Read Units", Placeholder: "New sustained read throughput limit (supply all three limits together)"},
	{Name: "max_write_units", Type: core.ConnectionTypeString, Label: "Max Write Units", Placeholder: "New sustained write throughput limit (supply all three limits together)"},
	{Name: "max_storage_gb", Type: core.ConnectionTypeString, Label: "Max Storage (GB)", Placeholder: "New maximum storage in GB (supply all three limits together)"},
	{Name: "ddl_statement", Type: core.ConnectionTypeText, Label: "DDL Statement", Placeholder: "Complete ALTER TABLE … statement (optional)"},
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
	tableID, err := ns.RequiredString("table_ocid_or_name", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}

	details := nosql.UpdateTableDetails{}
	// CompartmentId is required when the table is identified by name, and must match when an OCID is
	// given — so only send it for the name case to avoid a needless mismatch rejection.
	if !strings.HasPrefix(strings.ToLower(tableID), "ocid1.") {
		details.CompartmentId = &compartment
	}
	if ddl := ns.OptionalString("ddl_statement", inputs); strings.TrimSpace(ddl) != "" {
		details.DdlStatement = &ddl
	}

	// TableLimits requires all three sub-fields together (each is mandatory in the SDK), so treat the
	// limits as one all-or-nothing change: any one supplied ⇒ all three must be.
	readN, readOk, err := ns.OptionalInt("max_read_units", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	writeN, writeOk, err := ns.OptionalInt("max_write_units", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	storageN, storageOk, err := ns.OptionalInt("max_storage_gb", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	if anyLimit := readOk || writeOk || storageOk; anyLimit {
		if !(readOk && writeOk && storageOk) {
			return ns.ErrorResult("to change table limits, supply all three of max read units, max write units and max storage (GB)"), nil
		}
		details.TableLimits = &nosql.TableLimits{MaxReadUnits: &readN, MaxWriteUnits: &writeN, MaxStorageInGBs: &storageN}
	}

	if details.DdlStatement == nil && details.TableLimits == nil {
		return ns.ErrorResult("nothing to update — supply a DDL statement and/or new table limits"), nil
	}

	resp, err := client.UpdateTable(ns.Context(), nosql.UpdateTableRequest{TableNameOrId: &tableID, UpdateTableDetails: details})
	if err != nil {
		return ns.ErrorResult(auth.OCIError(err)), nil
	}
	return ns.Result(fmt.Sprintf("Updating table %q — poll Get Table until the change is applied", tableID), map[string]interface{}{
		"id":              tableID,
		"work_request_id": ns.Str(resp.OpcWorkRequestId),
	}), nil
}
