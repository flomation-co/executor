// Package oracle_mysql_db_system_stop stops a running MySQL HeatWave DB system by its OCID,
// using the chosen InnoDB shutdown mode. Asynchronous: the request returns a work-request id you
// can poll until the DB system is INACTIVE.
package oracle_mysql_db_system_stop

import (
	"fmt"

	core "flomation.app/automate/executor"
	my "flomation.app/automate/executor/actions/oracle/mysql"

	"github.com/oracle/oci-go-sdk/v65/mysql"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI MySQL: Stop DB System"
	Description  = "Stop a running MySQL HeatWave DB system by its OCID, choosing the InnoDB shutdown mode. Asynchronous — returns a work-request id you can poll until the DB system is INACTIVE."
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
	{Name: "db_system_ocid", Type: core.ConnectionTypeString, Label: "DB System OCID", Placeholder: "ocid1.mysqldbsystem.oc1..aaaa… of the DB system to stop", Required: true},
	{Name: "shutdown_type", Type: core.ConnectionTypeString, Label: "Shutdown Type", Placeholder: "The InnoDB shutdown mode", Required: true, Options: []core.ConnectionOption{
		{Name: "Fast (default clean shutdown)", Value: "FAST"},
		{Name: "Slow (full purge before shutdown)", Value: "SLOW"},
		{Name: "Immediate (skip purge — fastest)", Value: "IMMEDIATE"},
	}},
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
	shutdownRaw, err := my.RequiredString("shutdown_type", inputs)
	if err != nil {
		return my.ErrorResult(err.Error()), nil
	}
	shutdownType, ok := mysql.GetMappingInnoDbShutdownModeEnum(shutdownRaw)
	if !ok {
		return my.ErrorResult("shutdown type must be one of FAST, SLOW or IMMEDIATE"), nil
	}

	resp, err := client.StopDbSystem(my.Context(), mysql.StopDbSystemRequest{
		DbSystemId:          &dbSystemID,
		StopDbSystemDetails: mysql.StopDbSystemDetails{ShutdownType: shutdownType},
	})
	if err != nil {
		return my.ErrorResult(auth.OCIError(err)), nil
	}

	workRequestID := my.Str(resp.OpcWorkRequestId)
	msg := fmt.Sprintf("Stopping DB system %s (%s shutdown)", dbSystemID, shutdownType)
	if workRequestID != "" {
		msg = fmt.Sprintf("Stopping DB system %s (%s shutdown) — poll work request %s until the DB system is INACTIVE", dbSystemID, shutdownType, workRequestID)
	}
	return my.Result(msg, map[string]interface{}{
		"id":              dbSystemID,
		"work_request_id": workRequestID,
	}), nil
}
