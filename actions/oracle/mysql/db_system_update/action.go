// Package oracle_mysql_db_system_update applies a partial update to a MySQL HeatWave DB system:
// only the display name and description you supply are changed; blank fields are left as-is.
// Asynchronous — the call returns a work-request id you can poll until the update completes.
package oracle_mysql_db_system_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	my "flomation.app/automate/executor/actions/oracle/mysql"

	"github.com/oracle/oci-go-sdk/v65/mysql"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI MySQL: Update DB System"
	Description  = "Partially update a MySQL HeatWave DB system — change only the display name or description you supply; blank fields are left unchanged. Asynchronous: returns a work-request id to poll."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+database"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "db_system_ocid", Type: core.ConnectionTypeString, Label: "DB System OCID", Placeholder: "ocid1.mysqldbsystem.oc1..aaaa… — the DB system to update", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name (leave blank to keep unchanged)"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "New description (leave blank to keep unchanged)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "DB System OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := my.DbSystemClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	dbSystemID, err := my.RequiredString("db_system_ocid", inputs)
	if err != nil {
		return my.ErrorResult(err.Error()), nil
	}

	// Partial update: only carry the fields the operator actually supplied; blanks are left unchanged.
	var details mysql.UpdateDbSystemDetails
	if v := my.OptionalString("display_name", inputs); v != "" {
		details.DisplayName = &v
	}
	if v := my.OptionalString("description", inputs); v != "" {
		details.Description = &v
	}

	resp, err := client.UpdateDbSystem(my.Context(), mysql.UpdateDbSystemRequest{
		DbSystemId:            &dbSystemID,
		UpdateDbSystemDetails: details,
	})
	if err != nil {
		return my.ErrorResult(auth.OCIError(err)), nil
	}
	return my.Result(fmt.Sprintf("Updating DB system %s — poll the work request until it completes", dbSystemID), map[string]interface{}{
		"id":              dbSystemID,
		"work_request_id": my.Str(resp.OpcWorkRequestId),
	}), nil
}
