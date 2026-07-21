// Package oracle_blockvolume_backup_policy_create creates a custom, user-defined
// volume backup policy — a compartment-scoped set of backup schedules (frequency +
// retention) that can then be assigned to volumes, optionally copying scheduled
// backups to a paired destination region.
package oracle_blockvolume_backup_policy_create

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
	Name         = "OCI Block Volumes: Create Backup Policy"
	Description  = "Create a custom, user-defined volume backup policy in a compartment — a set of backup schedules (backup type, frequency period, and retention), optionally copying scheduled backups to a paired destination region."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "schedules_json", Type: core.ConnectionTypeString, Label: "Schedules (JSON array)", Placeholder: `[{"backupType":"INCREMENTAL","period":"ONE_DAY","retentionSeconds":604800,"offsetType":"STRUCTURED","hourOfDay":2,"timeZone":"REGIONAL_DATA_CENTER_TIME"}] — offsetType (STRUCTURED or NUMERIC_SECONDS) is required (optional)`},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Friendly name shown in the console (optional)"},
	{Name: "destination_region", Type: core.ConnectionTypeString, Label: "Destination Region", Placeholder: "Paired region to copy scheduled backups to, e.g. us-ashburn-1 (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "policy", Type: core.ConnectionTypeObject, Label: "Backup Policy"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Backup Policy OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := bv.GetAuth(inputs)
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}
	tags, err := bv.FreeformTags("tags", inputs)
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}
	details := ocicore.CreateVolumeBackupPolicyDetails{CompartmentId: &compartment, FreeformTags: tags}
	if v := strings.TrimSpace(bv.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if v := strings.ToLower(strings.TrimSpace(bv.OptionalString("destination_region", inputs))); v != "" {
		details.DestinationRegion = &v
	}
	if raw := strings.TrimSpace(bv.OptionalString("schedules_json", inputs)); raw != "" {
		var schedules []ocicore.VolumeBackupSchedule
		if err := json.Unmarshal([]byte(raw), &schedules); err != nil {
			return bv.ErrorResult(fmt.Sprintf("schedules must be a JSON array of schedule objects, e.g. [{\"backupType\":\"INCREMENTAL\",\"period\":\"ONE_DAY\",\"retentionSeconds\":604800}]: %s", err.Error())), nil
		}
		details.Schedules = schedules
	}

	client, err := auth.BlockstorageClient()
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	resp, err := client.CreateVolumeBackupPolicy(bv.Context(), ocicore.CreateVolumeBackupPolicyRequest{CreateVolumeBackupPolicyDetails: details})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	policy := bv.SummariseBackupPolicy(&resp.VolumeBackupPolicy)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created backup policy %q (%s)", policy["display_name"], policy["id"]),
		"policy":      policy,
		"id":          policy["id"],
		"success":     true,
	}, nil
}
