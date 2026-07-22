// Package oracle_exadata_db_node_update updates a database node (one of the VMs in a VM
// cluster) by OCID — currently its freeform tags. Asynchronous: it returns the node in an
// UPDATING state plus a work-request id; poll the Get DB Node action until the lifecycle
// state settles.
package oracle_exadata_db_node_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	exa "flomation.app/automate/executor/actions/oracle/exadata"

	db "github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Exadata: Update DB Node"
	Description  = "Update a database node (one of the VMs in a VM cluster) by OCID — set its freeform tags. Asynchronous — poll Get DB Node until the state settles."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+microchip"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "db_node_ocid", Type: core.ConnectionTypeString, Label: "DB Node OCID", Placeholder: "ocid1.dbnode.oc1..aaaa…", Required: true},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "db_node", Type: core.ConnectionTypeObject, Label: "DB Node"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "DB Node OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := exa.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	id, err := exa.RequiredString("db_node_ocid", inputs)
	if err != nil {
		return exa.ErrorResult(err.Error()), nil
	}

	details := db.UpdateDbNodeDetails{}
	if tags, err := exa.FreeformTags("tags", inputs); err != nil {
		return exa.ErrorResult(err.Error()), nil
	} else if tags != nil {
		details.FreeformTags = tags
	}

	resp, err := client.UpdateDbNode(exa.Context(), db.UpdateDbNodeRequest{DbNodeId: &id, UpdateDbNodeDetails: details})
	if err != nil {
		return exa.ErrorResult(auth.OCIError(err)), nil
	}
	dbNode := exa.SummariseDbNode(&resp.DbNode)
	return exa.Result(fmt.Sprintf("Updating DB node %q — now %s", dbNode["hostname"], dbNode["lifecycle_state"]), map[string]interface{}{
		"db_node":         dbNode,
		"id":              dbNode["id"],
		"lifecycle_state": dbNode["lifecycle_state"],
		"work_request_id": exa.Str(resp.OpcWorkRequestId),
	}), nil
}
