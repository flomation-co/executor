// Package oracle_nosql_index_delete deletes a secondary index from a NoSQL Database table.
// Asynchronous: the call returns a work-request id — poll it until the drop completes.
package oracle_nosql_index_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	ns "flomation.app/automate/executor/actions/oracle/nosql"

	"github.com/oracle/oci-go-sdk/v65/nosql"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI NoSQL: Delete Index"
	Description  = "Delete a secondary index from a NoSQL Database table by name. Asynchronous — returns a work-request id you can poll until the drop completes."
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
	{Name: "table_ocid_or_name", Type: core.ConnectionTypeString, Label: "Table OCID or Name", Placeholder: "The table's OCID, or its name within the compartment", Required: true},
	{Name: "index_name", Type: core.ConnectionTypeString, Label: "Index Name", Placeholder: "The name of the index to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Index Name"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := ns.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	table, err := ns.RequiredString("table_ocid_or_name", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	indexName, err := ns.RequiredString("index_name", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}

	req := nosql.DeleteIndexRequest{TableNameOrId: &table, IndexName: &indexName}
	if c := auth.CompartmentOCID; c != "" {
		req.CompartmentId = &c
	}

	resp, err := client.DeleteIndex(ns.Context(), req)
	if err != nil {
		return ns.ErrorResult(auth.OCIError(err)), nil
	}
	return ns.Result(fmt.Sprintf("Deleting index %q on table %q — poll the work request until it completes", indexName, table), map[string]interface{}{
		"id":              indexName,
		"work_request_id": ns.Str(resp.OpcWorkRequestId),
	}), nil
}
