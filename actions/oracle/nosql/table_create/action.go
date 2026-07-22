// Package oracle_nosql_table_create creates a NoSQL Database table from a CREATE TABLE DDL
// statement plus its throughput/storage limits. Asynchronous: the table comes back CREATING with a
// work-request id — poll Get Table (or the work request) until the table is ACTIVE before use.
package oracle_nosql_table_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	ns "flomation.app/automate/executor/actions/oracle/nosql"

	"github.com/oracle/oci-go-sdk/v65/nosql"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI NoSQL: Create Table"
	Description  = "Create a NoSQL Database table from a CREATE TABLE DDL statement and its read/write/storage limits. Returns a work-request id — poll Get Table until ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+table"
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Table Name", Placeholder: "The table name (must match the DDL)", Required: true},
	{Name: "ddl_statement", Type: core.ConnectionTypeText, Label: "CREATE TABLE Statement", Placeholder: "CREATE TABLE my_table (id INTEGER, name STRING, PRIMARY KEY(id))", Required: true},
	{Name: "max_read_units", Type: core.ConnectionTypeInteger, Label: "Max Read Units", Placeholder: "Maximum sustained read throughput (e.g. 50)", Required: true},
	{Name: "max_write_units", Type: core.ConnectionTypeInteger, Label: "Max Write Units", Placeholder: "Maximum sustained write throughput (e.g. 50)", Required: true},
	{Name: "max_storage_gb", Type: core.ConnectionTypeInteger, Label: "Max Storage (GB)", Placeholder: "Maximum storage in GB (e.g. 25)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
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
	name, err := ns.RequiredString("name", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	ddl, err := ns.RequiredString("ddl_statement", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	maxRead, err := ns.RequiredInt("max_read_units", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	maxWrite, err := ns.RequiredInt("max_write_units", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	maxStorage, err := ns.RequiredInt("max_storage_gb", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}

	details := nosql.CreateTableDetails{
		Name:          &name,
		CompartmentId: &compartment,
		DdlStatement:  &ddl,
		TableLimits: &nosql.TableLimits{
			MaxReadUnits:    &maxRead,
			MaxWriteUnits:   &maxWrite,
			MaxStorageInGBs: &maxStorage,
			CapacityMode:    nosql.TableLimitsCapacityModeProvisioned,
		},
	}

	resp, err := client.CreateTable(ns.Context(), nosql.CreateTableRequest{CreateTableDetails: details})
	if err != nil {
		return ns.ErrorResult(auth.OCIError(err)), nil
	}
	return ns.Result(fmt.Sprintf("Creating NoSQL table %q — poll Get Table until ACTIVE", name), map[string]interface{}{
		"work_request_id": ns.Str(resp.OpcWorkRequestId),
	}), nil
}
