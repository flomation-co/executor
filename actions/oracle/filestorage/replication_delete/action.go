// Package oracle_filestorage_replication_delete deletes a cross-region file-system
// replication by OCID.
package oracle_filestorage_replication_delete

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	fss "flomation.app/automate/executor/actions/oracle/filestorage"

	filestorage "github.com/oracle/oci-go-sdk/v65/filestorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI File Storage: Delete Replication"
	Description  = "Delete an Oracle Cloud File Storage replication by OCID. Optionally choose a delete mode to control how the in-flight delta cycle is handled before removal. Poll Get Replication (it 404s once gone)."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+copy"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the replication picker)"},
	{Name: "replication_ocid", Type: core.ConnectionTypeString, Label: "Replication OCID", Placeholder: "ocid1.filesystemreplication.oc1..aaaa…", Required: true},
	{Name: "delete_mode", Type: core.ConnectionTypeString, Label: "Delete Mode", Placeholder: "Optional — FINISH_CYCLE_IF_CAPTURING_OR_APPLYING (safest, default) | ONE_MORE_CYCLE (lossless failover) | FINISH_CYCLE_IF_APPLYING (fastest)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Replication OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := fss.ResourceClient(inputs, "replication_ocid")
	if errResult != nil {
		return errResult, nil
	}

	req := filestorage.DeleteReplicationRequest{ReplicationId: &id}
	if mode := strings.TrimSpace(fss.OptionalString("delete_mode", inputs)); mode != "" {
		enum, ok := filestorage.GetMappingDeleteReplicationDeleteModeEnum(mode)
		if !ok {
			return fss.ErrorResult(fmt.Sprintf("delete mode %q is not valid — expected one of: %s", mode, strings.Join(filestorage.GetDeleteReplicationDeleteModeEnumStringValues(), ", "))), nil
		}
		req.DeleteMode = enum
	}

	if _, err := client.DeleteReplication(fss.Context(), req); err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}
	return fss.Result(fmt.Sprintf("Deleted replication %s", id), map[string]interface{}{"id": id}), nil
}
