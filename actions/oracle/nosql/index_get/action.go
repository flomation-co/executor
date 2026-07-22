// Package oracle_nosql_index_get fetches a single secondary index on a NoSQL Database table by its
// name, returning the index's key columns and lifecycle state. The table may be given by name or
// OCID; a compartment helps disambiguate a name-based lookup.
package oracle_nosql_index_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	ns "flomation.app/automate/executor/actions/oracle/nosql"

	"github.com/oracle/oci-go-sdk/v65/nosql"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI NoSQL: Get Index"
	Description  = "Fetch a single secondary index on a NoSQL Database table by name — its key columns and lifecycle state."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (needed to interpret a table name — optional)"},
	{Name: "table_ocid_or_name", Type: core.ConnectionTypeString, Label: "Table Name or OCID", Placeholder: "A table name within the compartment, or a table OCID", Required: true},
	{Name: "index_name", Type: core.ConnectionTypeString, Label: "Index Name", Placeholder: "The name of the table's index", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "index", Type: core.ConnectionTypeObject, Label: "Index"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Index Name"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := ns.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	tableNameOrID, err := ns.RequiredString("table_ocid_or_name", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	indexName, err := ns.RequiredString("index_name", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}

	req := nosql.GetIndexRequest{TableNameOrId: &tableNameOrID, IndexName: &indexName}
	if auth.CompartmentOCID != "" {
		req.CompartmentId = &auth.CompartmentOCID
	}

	resp, err := client.GetIndex(ns.Context(), req)
	if err != nil {
		return ns.ErrorResult(auth.OCIError(err)), nil
	}
	index := ns.SummariseIndex(&resp.Index)
	return ns.Result(fmt.Sprintf("Index %q (%s)", index["name"], index["lifecycle_state"]), map[string]interface{}{
		"index": index, "name": index["name"], "lifecycle_state": index["lifecycle_state"],
	}), nil
}
