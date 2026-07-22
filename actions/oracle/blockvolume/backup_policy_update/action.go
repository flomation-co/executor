// Package oracle_blockvolume_backup_policy_update updates a user-defined block-
// volume backup policy — its display name, backup schedules, and the paired
// cross-region copy destination.
package oracle_blockvolume_backup_policy_update

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	bv "flomation.app/automate/executor/actions/oracle/blockvolume"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Block Volumes: Update Backup Policy"
	Description  = "Update a user-defined Oracle Cloud block-volume backup policy — rename it, replace its backup schedules, or change its paired cross-region copy destination."
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
	{Name: "policy_ocid", Type: core.ConnectionTypeString, Label: "Backup Policy OCID", Placeholder: "ocid1.volumebackuppolicy.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New friendly name for the policy (optional)"},
	{Name: "schedules_json", Type: core.ConnectionTypeString, Label: "Schedules (JSON array)", Placeholder: `[{"backupType":"INCREMENTAL","period":"ONE_DAY","retentionSeconds":604800,"offsetType":"STRUCTURED","hourOfDay":2,"timeZone":"REGIONAL_DATA_CENTER_TIME"}] — REPLACES all schedules; offsetType required (optional)`},
	{Name: "destination_region", Type: core.ConnectionTypeString, Label: "Destination Region", Placeholder: "Paired region for scheduled copies, e.g. us-ashburn-1; 'none' to clear (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "policy", Type: core.ConnectionTypeObject, Label: "Backup Policy"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Policy OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := bv.VolumeResourceClient(inputs, "policy_ocid")
	if errResult != nil {
		return errResult, nil
	}

	details := ocicore.UpdateVolumeBackupPolicyDetails{}
	if v := strings.TrimSpace(bv.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if v := bv.NormaliseRegion("destination_region", inputs); v != "" {
		details.DestinationRegion = &v
	}
	// schedules_json REPLACES the policy's entire schedule set (REPLACE semantics) —
	// parse it only when the operator supplied one, otherwise leave Schedules nil so
	// the policy's existing schedules are preserved.
	if raw := strings.TrimSpace(bv.OptionalString("schedules_json", inputs)); raw != "" {
		var schedules []ocicore.VolumeBackupSchedule
		if err := json.Unmarshal([]byte(raw), &schedules); err != nil {
			return bv.ErrorResult(fmt.Sprintf("schedules must be a JSON array of schedule objects, e.g. [{\"backupType\":\"INCREMENTAL\",\"period\":\"ONE_DAY\",\"retentionSeconds\":604800}]: %s", err.Error())), nil
		}
		details.Schedules = schedules
	}

	resp, err := client.UpdateVolumeBackupPolicy(bv.Context(), ocicore.UpdateVolumeBackupPolicyRequest{
		PolicyId:                        &id,
		UpdateVolumeBackupPolicyDetails: details,
	})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	policy := bv.SummariseBackupPolicy(&resp.VolumeBackupPolicy)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Updated backup policy %q", policy["display_name"]),
		"policy":      policy,
		"id":          policy["id"],
		"success":     true,
	}, nil
}
