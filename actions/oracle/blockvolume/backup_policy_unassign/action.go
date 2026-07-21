// Package oracle_blockvolume_backup_policy_unassign removes a backup policy
// assignment from an asset, stopping the policy's scheduled backups for it.
package oracle_blockvolume_backup_policy_unassign

import (
	"fmt"

	core "flomation.app/automate/executor"
	bv "flomation.app/automate/executor/actions/oracle/blockvolume"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Block Volumes: Remove Backup Policy Assignment"
	Description  = "Detach a backup policy from an asset (a volume or boot volume) by assignment OCID, stopping its scheduled backups. Existing backups are not deleted."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+gear"
	Date         = "21/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the assignment picker)"},
	{Name: "assignment_ocid", Type: core.ConnectionTypeString, Label: "Assignment OCID", Placeholder: "ocid1.volumebackuppolicyassignment.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Assignment OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := bv.VolumeResourceClient(inputs, "assignment_ocid")
	if errResult != nil {
		return errResult, nil
	}
	if _, err := client.DeleteVolumeBackupPolicyAssignment(bv.Context(), ocicore.DeleteVolumeBackupPolicyAssignmentRequest{PolicyAssignmentId: &id}); err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Removed backup policy assignment %s", id),
		"id":          id,
		"success":     true,
	}, nil
}
