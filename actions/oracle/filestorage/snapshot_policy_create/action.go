// Package oracle_filestorage_snapshot_policy_create creates a file system snapshot
// policy — the schedule that automates snapshot creation and retention for the file
// systems associated with it. The policy is availability-domain-scoped. Synchronous-ish:
// the call returns the policy with its OCID in a CREATING state; poll Get Snapshot Policy
// until ACTIVE.
package oracle_filestorage_snapshot_policy_create

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
	Name         = "OCI File Storage: Create Snapshot Policy"
	Description  = "Create an Oracle Cloud file system snapshot policy — the schedule that automates snapshot creation and retention. Attach file systems to it to have snapshots taken on schedule. Returns the OCID immediately in a CREATING state; poll Get Snapshot Policy until ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+calendar"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A friendly name for the snapshot policy (optional)"},
	{Name: "policy_prefix", Type: core.ConnectionTypeString, Label: "Policy Prefix", Placeholder: "Prefix applied to every snapshot this policy creates, e.g. acme (optional)"},
	{Name: "schedules_json", Type: core.ConnectionTypeText, Label: "Schedules (JSON array)", Placeholder: `[{"timeZone":"UTC","period":"DAILY","hourOfDay":18,"retentionDurationInSeconds":604800}] — max 10; period one of HOURLY/DAILY/WEEKLY/MONTHLY/YEARLY (optional)`},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
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
	auth, client, errResult := fss.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	ad, err := fss.RequiredAvailabilityDomain(inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	details := filestorage.CreateFilesystemSnapshotPolicyDetails{CompartmentId: &compartment, AvailabilityDomain: &ad}
	if v := strings.TrimSpace(fss.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if v := strings.TrimSpace(fss.OptionalString("policy_prefix", inputs)); v != "" {
		details.PolicyPrefix = &v
	}
	if raw := strings.TrimSpace(fss.OptionalString("schedules_json", inputs)); raw != "" {
		var schedules []filestorage.SnapshotSchedule
		if err := json.Unmarshal([]byte(raw), &schedules); err != nil {
			return fss.ErrorResult(fmt.Sprintf("schedules must be a JSON array of schedule objects, e.g. [{\"timeZone\":\"UTC\",\"period\":\"DAILY\",\"hourOfDay\":18}]: %s", err.Error())), nil
		}
		details.Schedules = schedules
	}
	if tags, err := fss.FreeformTags("tags", inputs); err != nil {
		return fss.ErrorResult(err.Error()), nil
	} else {
		details.FreeformTags = tags
	}
	resp, err := client.CreateFilesystemSnapshotPolicy(fss.Context(), filestorage.CreateFilesystemSnapshotPolicyRequest{CreateFilesystemSnapshotPolicyDetails: details})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}
	policy := summarisePolicy(&resp.FilesystemSnapshotPolicy)
	name := fss.Str(resp.FilesystemSnapshotPolicy.DisplayName)
	if name == "" {
		name = fss.Str(resp.FilesystemSnapshotPolicy.Id)
	}
	return fss.Result(fmt.Sprintf("Creating snapshot policy %q (%s) — poll Get Snapshot Policy until ACTIVE", name, policy["lifecycle_state"]), map[string]interface{}{
		"snapshot_policy": policy, "id": policy["id"], "lifecycle_state": policy["lifecycle_state"],
	}), nil
}

func summarisePolicy(p *filestorage.FilesystemSnapshotPolicy) map[string]interface{} {
	return map[string]interface{}{
		"id":                  fss.Str(p.Id),
		"display_name":        fss.Str(p.DisplayName),
		"compartment_id":      fss.Str(p.CompartmentId),
		"availability_domain": fss.Str(p.AvailabilityDomain),
		"lifecycle_state":     string(p.LifecycleState),
		"policy_prefix":       fss.Str(p.PolicyPrefix),
		"schedules":           summariseSchedules(p.Schedules),
		"time_created":        fss.FormatTime(p.TimeCreated),
	}
}

func summariseSchedules(schedules []filestorage.SnapshotSchedule) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(schedules))
	for i := range schedules {
		s := schedules[i]
		m := map[string]interface{}{
			"period":              string(s.Period),
			"time_zone":           string(s.TimeZone),
			"schedule_prefix":     fss.Str(s.SchedulePrefix),
			"time_schedule_start": fss.FormatTime(s.TimeScheduleStart),
		}
		if s.RetentionDurationInSeconds != nil {
			m["retention_duration_in_seconds"] = *s.RetentionDurationInSeconds
		}
		if s.HourOfDay != nil {
			m["hour_of_day"] = *s.HourOfDay
		}
		if s.DayOfMonth != nil {
			m["day_of_month"] = *s.DayOfMonth
		}
		if s.DayOfWeek != "" {
			m["day_of_week"] = string(s.DayOfWeek)
		}
		if s.Month != "" {
			m["month"] = string(s.Month)
		}
		out = append(out, m)
	}
	return out
}
