// Package oracle_filestorage_snapshot_policy_update renames / re-tags a file system
// snapshot policy and (optionally) replaces its snapshot schedules.
package oracle_filestorage_snapshot_policy_update

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	fss "flomation.app/automate/executor/actions/oracle/filestorage"

	filestorage "github.com/oracle/oci-go-sdk/v65/filestorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI File Storage: Update Snapshot Policy"
	Description  = "Update an Oracle Cloud file system snapshot policy — its display name, snapshot prefix and/or the list of snapshot schedules. Leave schedules blank to keep the current ones untouched."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+calendar"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the snapshot-policy picker)"},
	{Name: "snapshot_policy_ocid", Type: core.ConnectionTypeString, Label: "Snapshot Policy OCID", Placeholder: "ocid1.filesystemsnapshotpolicy.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name (leave blank to keep the current one)"},
	{Name: "policy_prefix", Type: core.ConnectionTypeString, Label: "Policy Prefix", Placeholder: "Prefix applied to every snapshot this policy creates (leave blank to keep)"},
	{Name: "schedules_json", Type: core.ConnectionTypeText, Label: "Schedules (JSON array)", Placeholder: `[{"timeZone":"UTC","period":"DAILY","hourOfDay":18},{"timeZone":"UTC","period":"HOURLY"}] — fully replaces the schedule list; leave blank to keep the current schedules`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "snapshot_policy", Type: core.ConnectionTypeObject, Label: "Snapshot Policy"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Snapshot Policy OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := fss.ResourceClient(inputs, "snapshot_policy_ocid")
	if errResult != nil {
		return errResult, nil
	}

	details := filestorage.UpdateFilesystemSnapshotPolicyDetails{}
	changed := false

	if name := strings.TrimSpace(fss.OptionalString("display_name", inputs)); name != "" {
		details.DisplayName = &name
		changed = true
	}

	if prefix := strings.TrimSpace(fss.OptionalString("policy_prefix", inputs)); prefix != "" {
		details.PolicyPrefix = &prefix
		changed = true
	}

	schedulesRaw := strings.TrimSpace(fss.OptionalString("schedules_json", inputs))
	if schedulesRaw != "" {
		var scheds []filestorage.SnapshotSchedule
		if err := json.Unmarshal([]byte(schedulesRaw), &scheds); err != nil {
			return fss.ErrorResult(fmt.Sprintf(`schedules must be a JSON array of snapshot schedules, e.g. [{"timeZone":"UTC","period":"DAILY","hourOfDay":18}]: %s`, err.Error())), nil
		}
		details.Schedules = scheds
		changed = true
	}

	if !changed {
		return fss.ErrorResult("nothing to update — supply a display name, policy prefix and/or schedules"), nil
	}

	// UpdateFilesystemSnapshotPolicy REPLACES the full schedule list. When the operator
	// leaves schedules blank, read the current policy and re-send its schedules so the
	// update does not silently wipe them (read-modify-write).
	if schedulesRaw == "" {
		cur, err := client.GetFilesystemSnapshotPolicy(fss.Context(), filestorage.GetFilesystemSnapshotPolicyRequest{FilesystemSnapshotPolicyId: &id})
		if err != nil {
			return fss.ErrorResult(auth.OCIError(err)), nil
		}
		details.Schedules = cur.FilesystemSnapshotPolicy.Schedules
	}

	resp, err := client.UpdateFilesystemSnapshotPolicy(fss.Context(), filestorage.UpdateFilesystemSnapshotPolicyRequest{
		FilesystemSnapshotPolicyId:            &id,
		UpdateFilesystemSnapshotPolicyDetails: details,
	})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}

	policy := summarisePolicy(&resp.FilesystemSnapshotPolicy)
	return fss.Result(fmt.Sprintf("Updated snapshot policy %q (%s)", policy["display_name"], policy["lifecycle_state"]), map[string]interface{}{
		"snapshot_policy": policy, "id": policy["id"], "lifecycle_state": policy["lifecycle_state"],
	}), nil
}

// summarisePolicy shapes a FilesystemSnapshotPolicy into the result map (no shared
// summariser exists for this long-tail type).
func summarisePolicy(p *filestorage.FilesystemSnapshotPolicy) map[string]interface{} {
	return map[string]interface{}{
		"id":                  fss.Str(p.Id),
		"display_name":        fss.Str(p.DisplayName),
		"compartment_id":      fss.Str(p.CompartmentId),
		"availability_domain": fss.Str(p.AvailabilityDomain),
		"policy_prefix":       fss.Str(p.PolicyPrefix),
		"lifecycle_state":     string(p.LifecycleState),
		"schedules":           summariseSchedules(p.Schedules),
		"time_created":        fss.FormatTime(p.TimeCreated),
	}
}

func summariseSchedules(scheds []filestorage.SnapshotSchedule) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(scheds))
	for i := range scheds {
		s := scheds[i]
		out = append(out, map[string]interface{}{
			"period":                        string(s.Period),
			"time_zone":                     string(s.TimeZone),
			"schedule_prefix":               fss.Str(s.SchedulePrefix),
			"time_schedule_start":           fss.FormatTime(s.TimeScheduleStart),
			"retention_duration_in_seconds": fss.Int64OrNil(s.RetentionDurationInSeconds),
			"hour_of_day":                   intOrNil(s.HourOfDay),
			"day_of_week":                   string(s.DayOfWeek),
			"day_of_month":                  intOrNil(s.DayOfMonth),
			"month":                         string(s.Month),
		})
	}
	return out
}

func intOrNil(p *int) interface{} {
	if p == nil {
		return nil
	}
	return *p
}
