// Package oracle_autonomousdatabase_db_update updates editable attributes of an
// Oracle Cloud Autonomous Database — display name, ADMIN password, license model
// or allowed IPs. The operation is asynchronous — it returns immediately with a
// work-request id while the database applies the change.
package oracle_autonomousdatabase_db_update

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
	Name         = "OCI Autonomous Database: Update Attributes"
	Description  = "Update editable attributes of an Oracle Cloud Autonomous Database — display name, ADMIN password, license model or allowed IPs. Asynchronous — returns a work-request id."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+pen"
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
	{Name: "autonomous_database_id", Type: core.ConnectionTypeString, Label: "Autonomous Database OCID", Placeholder: "ocid1.autonomousdatabase.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New friendly name shown in the console (optional)"},
	{Name: "admin_password", Type: core.ConnectionTypeSecret, Label: "ADMIN Password", Placeholder: "New ADMIN password — 12–30 chars, incl. upper, lower and a digit (optional)"},
	{Name: "license_model", Type: core.ConnectionTypeString, Label: "License Model", Placeholder: "Optional", Options: []core.ConnectionOption{
		{Name: "License Included", Value: "LICENSE_INCLUDED"},
		{Name: "Bring Your Own License", Value: "BRING_YOUR_OWN_LICENSE"},
	}},
	{Name: "whitelisted_ips", Type: core.ConnectionTypeString, Label: "Allowed IPs", Placeholder: "Comma-separated IPs/CIDRs to allow (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "database", Type: core.ConnectionTypeObject, Label: "Database"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := adb.PerDatabaseClient(inputs)
	if errResult != nil {
		return errResult, nil
	}

	details := database.UpdateAutonomousDatabaseDetails{}
	if v := strings.TrimSpace(adb.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if v := adb.OptionalString("admin_password", inputs); v != "" {
		details.AdminPassword = &v
	}
	if v := strings.TrimSpace(adb.OptionalString("license_model", inputs)); v != "" {
		details.LicenseModel = database.UpdateAutonomousDatabaseDetailsLicenseModelEnum(v)
	}
	if ips := adb.InputStrings("whitelisted_ips", inputs); len(ips) > 0 {
		details.WhitelistedIps = ips
	}

	resp, err := client.UpdateAutonomousDatabase(adb.Context(), database.UpdateAutonomousDatabaseRequest{
		AutonomousDatabaseId:            &id,
		UpdateAutonomousDatabaseDetails: details,
	})
	if err != nil {
		return adb.ErrorResult(auth.OCIError(err)), nil
	}
	db := adb.SummariseAutonomousDatabase(&resp.AutonomousDatabase)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Update requested for Autonomous Database %q (now %s)", db["display_name"], db["lifecycle_state"]),
		"database":        db,
		"lifecycle_state": db["lifecycle_state"],
		"work_request_id": adb.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
