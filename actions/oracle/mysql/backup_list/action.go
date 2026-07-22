// Package oracle_mysql_backup_list lists the MySQL HeatWave backups in a compartment, optionally
// filtered by DB system, exact display name or lifecycle state. Walks pagination up to a safe cap.
package oracle_mysql_backup_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	my "flomation.app/automate/executor/actions/oracle/mysql"

	"github.com/oracle/oci-go-sdk/v65/mysql"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI MySQL: List Backups"
	Description  = "List the MySQL HeatWave backups in a compartment. Optionally filter by DB system, exact display name or lifecycle state. Walks pagination up to a safe cap."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "db_system_id", Type: core.ConnectionTypeString, Label: "DB System OCID", Placeholder: "Only backups of this DB system (optional)"},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name Filter", Placeholder: "Only backups with this exact name (optional)"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State", Placeholder: "Filter by state (optional)", Options: []core.ConnectionOption{
		{Name: "Creating", Value: "CREATING"},
		{Name: "Active", Value: "ACTIVE"},
		{Name: "Inactive", Value: "INACTIVE"},
		{Name: "Updating", Value: "UPDATING"},
		{Name: "Deleting", Value: "DELETING"},
		{Name: "Deleted", Value: "DELETED"},
		{Name: "Failed", Value: "FAILED"},
		{Name: "Delete Scheduled", Value: "DELETE_SCHEDULED"},
	}},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Max items per API page (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "backups", Type: core.ConnectionTypeObject, Label: "Backups"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := my.BackupsClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return my.ErrorResult(err.Error()), nil
	}

	req := mysql.ListBackupsRequest{CompartmentId: &compartment}
	if dbSys := my.OptionalString("db_system_id", inputs); dbSys != "" {
		req.DbSystemId = &dbSys
	}
	if name := my.OptionalString("display_name", inputs); name != "" {
		req.DisplayName = &name
	}
	if state := my.OptionalString("lifecycle_state", inputs); state != "" {
		req.LifecycleState = mysql.BackupLifecycleStateEnum(state)
	}
	if limit, ok, err := my.OptionalInt("limit", inputs); err != nil {
		return my.ErrorResult(err.Error()), nil
	} else if ok {
		req.Limit = &limit
	}

	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= my.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListBackups(my.Context(), req)
		if err != nil {
			return my.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, my.SummariseBackupSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}

	return my.Result(fmt.Sprintf("Found %d backup(s)", len(out)), map[string]interface{}{
		"backups": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
