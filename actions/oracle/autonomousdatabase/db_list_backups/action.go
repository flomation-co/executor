// Package oracle_autonomousdatabase_db_list_backups lists the backups of one
// Oracle Cloud Autonomous Database by OCID, or every backup in a compartment.
package oracle_autonomousdatabase_db_list_backups

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
	Name         = "OCI Autonomous Database: List Backups"
	Description  = "List the backups of an Oracle Cloud Autonomous Database, or all backups in a compartment. Provide a database OCID or a compartment."
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
	{Name: "autonomous_database_id", Type: core.ConnectionTypeString, Label: "Autonomous Database OCID", Placeholder: "ocid1.autonomousdatabase.oc1..aaaa… — backups of this one database (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… — every backup in this compartment (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "backups", Type: core.ConnectionTypeObject, Label: "Backups"},
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
	client, err := auth.DatabaseClient()
	if err != nil {
		return adb.ErrorResult(auth.OCIError(err)), nil
	}
	ctx := adb.Context()

	dbID := strings.TrimSpace(adb.OptionalString("autonomous_database_id", inputs))
	compartment := strings.TrimSpace(adb.OptionalString("compartment_ocid", inputs))
	if dbID == "" && compartment == "" {
		return adb.ErrorResult("provide an autonomous database OCID or a compartment OCID"), nil
	}

	req := database.ListAutonomousDatabaseBackupsRequest{}
	if dbID != "" {
		req.AutonomousDatabaseId = &dbID
	}
	if compartment != "" {
		req.CompartmentId = &compartment
	}

	var backups []map[string]interface{}
	truncated := false
	for page := 0; page < adb.ListMaxPages; page++ {
		resp, err := client.ListAutonomousDatabaseBackups(ctx, req)
		if err != nil {
			return adb.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			backups = append(backups, adb.SummariseBackupSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
		if page == adb.ListMaxPages-1 {
			truncated = true
		}
	}

	summary := fmt.Sprintf("Found %d Autonomous Database backup(s)", len(backups))
	if truncated {
		summary = fmt.Sprintf("Found at least %d Autonomous Database backup(s) (list truncated at %d pages — more available)", len(backups), adb.ListMaxPages)
	}
	return map[string]interface{}{
		"tool_result": summary,
		"backups":     backups,
		"count":       len(backups),
		"truncated":   truncated,
		"success":     true,
	}, nil
}
