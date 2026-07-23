// Package oracle_autonomousdatabase_db_get_all lists the Autonomous Databases in
// an Oracle Cloud compartment, optionally filtered by display name or workload.
package oracle_autonomousdatabase_db_get_all

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	adb "flomation.app/automate/executor/actions/oracle/autonomousdatabase"

	"github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Autonomous Database: List"
	Description  = "List the Autonomous Databases in an Oracle Cloud compartment, with their lifecycle state, workload and sizing. Optionally filter by display name or workload type. Large compartments are capped — check the 'truncated' output to tell a complete listing from a partial one."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+list"
	Date         = "21/07/2026"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name Filter", Placeholder: "Only databases with this exact display name (optional)"},
	{Name: "db_workload", Type: core.ConnectionTypeString, Label: "Workload Filter", Placeholder: "Only this workload type (optional)", Options: []core.ConnectionOption{
		{Name: "Transaction Processing (OLTP)", Value: "OLTP"},
		{Name: "Data Warehouse (DW)", Value: "DW"},
		{Name: "JSON Database (AJD)", Value: "AJD"},
		{Name: "APEX", Value: "APEX"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "databases", Type: core.ConnectionTypeObject, Label: "Databases"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := adb.GetAuth(inputs)
	if err != nil {
		return adb.ErrorResult(err.Error()), nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return adb.ErrorResult(err.Error()), nil
	}
	client, err := auth.DatabaseClient()
	if err != nil {
		return adb.ErrorResult(auth.OCIError(err)), nil
	}
	ctx := adb.Context()

	req := database.ListAutonomousDatabasesRequest{CompartmentId: &compartment}
	if v := strings.TrimSpace(adb.OptionalString("display_name", inputs)); v != "" {
		req.DisplayName = &v
	}
	if v := strings.TrimSpace(adb.OptionalString("db_workload", inputs)); v != "" {
		req.DbWorkload = database.AutonomousDatabaseSummaryDbWorkloadEnum(v)
	}

	var dbs []map[string]interface{}
	truncated := false
	for page := 0; page < adb.ListMaxPages; page++ {
		resp, err := client.ListAutonomousDatabases(ctx, req)
		if err != nil {
			return adb.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			dbs = append(dbs, adb.SummariseAutonomousDatabaseSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
		if page == adb.ListMaxPages-1 {
			truncated = true
		}
	}

	summary := fmt.Sprintf("Found %d Autonomous Database(s) in the compartment", len(dbs))
	if truncated {
		summary = fmt.Sprintf("Found at least %d Autonomous Database(s) (list truncated at %d pages — more available)", len(dbs), adb.ListMaxPages)
	}
	return map[string]interface{}{
		"tool_result": summary,
		"databases":   dbs,
		"count":       len(dbs),
		"truncated":   truncated,
		"success":     true,
	}, nil
}
