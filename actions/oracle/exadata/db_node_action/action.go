// Package oracle_exadata_db_node_action starts, stops or resets a database node (one of the
// VMs in a VM cluster). Asynchronous — it returns the node in its new transitional state.
package oracle_exadata_db_node_action

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	exa "flomation.app/automate/executor/actions/oracle/exadata"

	db "github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Exadata: DB Node Action"
	Description  = "Start, stop or reset a database node (one of the VMs in a VM cluster). SOFTRESET does a graceful OS reboot; RESET is a hard power-cycle. Asynchronous — poll Get DB Node until the state settles."
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
	{Name: "action", Type: core.ConnectionTypeString, Label: "Action", Placeholder: "What to do to the node", Required: true, Options: []core.ConnectionOption{
		{Name: "Start", Value: "START"},
		{Name: "Stop", Value: "STOP"},
		{Name: "Soft reset (graceful reboot)", Value: "SOFTRESET"},
		{Name: "Reset (hard power-cycle)", Value: "RESET"},
	}},
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
	action := strings.ToUpper(strings.TrimSpace(exa.OptionalString("action", inputs)))
	switch action {
	case "START", "STOP", "SOFTRESET", "RESET":
	default:
		return exa.ErrorResult("action must be START, STOP, SOFTRESET or RESET"), nil
	}
	resp, err := client.DbNodeAction(exa.Context(), db.DbNodeActionRequest{DbNodeId: &id, Action: db.DbNodeActionActionEnum(action)})
	if err != nil {
		return exa.ErrorResult(auth.OCIError(err)), nil
	}
	dbNode := exa.SummariseDbNode(&resp.DbNode)
	return exa.Result(fmt.Sprintf("%s issued on DB node — it is now %s (poll Get DB Node until it settles)", action, dbNode["lifecycle_state"]), map[string]interface{}{
		"db_node": dbNode, "id": dbNode["id"], "lifecycle_state": dbNode["lifecycle_state"], "work_request_id": exa.Str(resp.OpcWorkRequestId),
	}), nil
}
