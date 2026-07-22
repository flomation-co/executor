// Package oracle_blockvolume_backup_policy_assign attaches a backup policy to a
// volume or boot volume (the "asset") so its backups are taken on a schedule. This
// is the headline of the node — automated, scheduled backups without a running flow.
package oracle_blockvolume_backup_policy_assign

import (
	"fmt"

	core "flomation.app/automate/executor"
	bv "flomation.app/automate/executor/actions/oracle/blockvolume"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Block Volumes: Assign Backup Policy"
	Description  = "Attach a backup policy to a block volume or boot volume so backups are taken automatically on the policy's schedule."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the policy picker)"},
	{Name: "asset_ocid", Type: core.ConnectionTypeString, Label: "Asset OCID", Placeholder: "The volume or boot-volume OCID to schedule backups for", Required: true},
	{Name: "policy_ocid", Type: core.ConnectionTypeString, Label: "Backup Policy OCID", Placeholder: "ocid1.volumebackuppolicy.oc1..aaaa… (predefined Bronze/Silver/Gold or your own)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "assignment", Type: core.ConnectionTypeObject, Label: "Policy Assignment"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Assignment OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	// compartment_ocid is declared only to scope the policy_ocid picker in the editor
	// (see the api's dynamicOptionsMetadata) — the assignment itself is keyed on the
	// asset + policy OCIDs, so it is intentionally not read here. Same pattern as the
	// networking node's per-resource actions.
	auth, err := bv.GetAuth(inputs)
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}
	asset, err := bv.RequiredString("asset_ocid", inputs)
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}
	policy, err := bv.RequiredString("policy_ocid", inputs)
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}
	client, err := auth.BlockstorageClient()
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	resp, err := client.CreateVolumeBackupPolicyAssignment(bv.Context(), ocicore.CreateVolumeBackupPolicyAssignmentRequest{
		CreateVolumeBackupPolicyAssignmentDetails: ocicore.CreateVolumeBackupPolicyAssignmentDetails{
			AssetId:  &asset,
			PolicyId: &policy,
		},
	})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	assignment := bv.SummariseAssignment(&resp.VolumeBackupPolicyAssignment)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Assigned backup policy %s to asset %s", policy, asset),
		"assignment":  assignment,
		"id":          assignment["id"],
		"success":     true,
	}, nil
}
