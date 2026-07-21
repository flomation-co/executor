// Package oracle_autonomousdatabase_db_list_clones lists the clones created from a
// source Oracle Cloud Autonomous Database within a compartment.
package oracle_autonomousdatabase_db_list_clones

import (
	"fmt"

	core "flomation.app/automate/executor"
	adb "flomation.app/automate/executor/actions/oracle/autonomousdatabase"

	"github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Autonomous Database: List Clones"
	Description  = "List the clones created from a source Oracle Cloud Autonomous Database within a compartment."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+list"
	Date         = "21/07/2026"
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
	{Name: "source_autonomous_database_id", Type: core.ConnectionTypeString, Label: "Source Autonomous Database OCID", Placeholder: "ocid1.autonomousdatabase.oc1..aaaa… (the database whose clones to list)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "databases", Type: core.ConnectionTypeObject, Label: "Clones"},
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
	source, err := adb.RequiredString("source_autonomous_database_id", inputs)
	if err != nil {
		return adb.ErrorResult(err.Error()), nil
	}
	client, err := auth.DatabaseClient()
	if err != nil {
		return adb.ErrorResult(auth.OCIError(err)), nil
	}
	ctx := adb.Context()

	req := database.ListAutonomousDatabaseClonesRequest{
		CompartmentId:        &compartment,
		AutonomousDatabaseId: &source,
	}

	var dbs []map[string]interface{}
	truncated := false
	for page := 0; page < adb.ListMaxPages; page++ {
		resp, err := client.ListAutonomousDatabaseClones(ctx, req)
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

	summary := fmt.Sprintf("Found %d clone(s) of the source Autonomous Database", len(dbs))
	if truncated {
		summary = fmt.Sprintf("Found at least %d clone(s) (list truncated at %d pages — more available)", len(dbs), adb.ListMaxPages)
	}
	return map[string]interface{}{
		"tool_result": summary,
		"databases":   dbs,
		"count":       len(dbs),
		"truncated":   truncated,
		"success":     true,
	}, nil
}
