// Package oracle_mysql_db_system_change_compartment would move a MySQL HeatWave DB system into a
// different compartment — but the OCI MySQL API exposes no such operation. DbSystemClient offers
// create/get/list/update/delete and start/stop/restart only; there is no ChangeDbSystemCompartment
// (unlike backups, which have ChangeBackupCompartment). So this action validates its inputs and
// returns a soft error explaining the limitation and the recreate-in-target workaround, rather than
// silently doing nothing.
package oracle_mysql_db_system_change_compartment

import (
	"fmt"

	core "flomation.app/automate/executor"
	my "flomation.app/automate/executor/actions/oracle/mysql"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI MySQL: Change DB System Compartment"
	Description  = "Attempt to move a MySQL HeatWave DB system to another compartment — reports that OCI provides no such operation for DB systems."
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
	{Name: "db_system_ocid", Type: core.ConnectionTypeString, Label: "DB System OCID", Placeholder: "ocid1.mysqldbsystem.oc1..aaaa… (the DB system to move)", Required: true},
	{Name: "destination_compartment_ocid", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (where you want the DB system)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	dbSystemID, err := my.RequiredString("db_system_ocid", inputs)
	if err != nil {
		return my.ErrorResult(err.Error()), nil
	}
	destination, err := my.RequiredString("destination_compartment_ocid", inputs)
	if err != nil {
		return my.ErrorResult(err.Error()), nil
	}

	return my.ErrorResult(fmt.Sprintf(
		"OCI MySQL HeatWave DB systems cannot be moved between compartments: the MySQL API has no change-compartment operation for DB systems (only backups can be moved). "+
			"To place DB system %s in compartment %s, recreate it there — typically by taking a backup, then creating a new DB system from that backup in the destination compartment — and delete the original.",
		dbSystemID, destination,
	)), nil
}
