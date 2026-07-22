// Package oracle_filestorage_snapshot_policy_get reads one file system snapshot policy by OCID.
package oracle_filestorage_snapshot_policy_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	fss "flomation.app/automate/executor/actions/oracle/filestorage"

	filestorage "github.com/oracle/oci-go-sdk/v65/filestorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI File Storage: Get Snapshot Policy"
	Description  = "Fetch a single Oracle Cloud file system snapshot policy by OCID — its display name, policy prefix, lifecycle state and the snapshot schedules it runs."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the snapshot-policy picker)"},
	{Name: "snapshot_policy_ocid", Type: core.ConnectionTypeString, Label: "Snapshot Policy OCID", Placeholder: "ocid1.filesystemsnapshotpolicy.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "snapshot_policy", Type: core.ConnectionTypeObject, Label: "Snapshot Policy"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Snapshot Policy OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func intOrNil(p *int) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

func summariseSchedule(s filestorage.SnapshotSchedule) map[string]interface{} {
	return map[string]interface{}{
		"period":                        string(s.Period),
		"time_zone":                     string(s.TimeZone),
		"schedule_prefix":               fss.Str(s.SchedulePrefix),
		"time_schedule_start":           fss.FormatTime(s.TimeScheduleStart),
		"retention_duration_in_seconds": fss.Int64OrNil(s.RetentionDurationInSeconds),
		"hour_of_day":                   intOrNil(s.HourOfDay),
		"day_of_week":                   string(s.DayOfWeek),
		"day_of_month":                  intOrNil(s.DayOfMonth),
		"month":                         string(s.Month),
	}
}

func summarisePolicy(p *filestorage.FilesystemSnapshotPolicy) map[string]interface{} {
	schedules := make([]map[string]interface{}, 0, len(p.Schedules))
	for i := range p.Schedules {
		schedules = append(schedules, summariseSchedule(p.Schedules[i]))
	}
	return map[string]interface{}{
		"id":                  fss.Str(p.Id),
		"display_name":        fss.Str(p.DisplayName),
		"compartment_id":      fss.Str(p.CompartmentId),
		"availability_domain": fss.Str(p.AvailabilityDomain),
		"policy_prefix":       fss.Str(p.PolicyPrefix),
		"lifecycle_state":     string(p.LifecycleState),
		"schedules":           schedules,
		"freeform_tags":       p.FreeformTags,
		"time_created":        fss.FormatTime(p.TimeCreated),
	}
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := fss.ResourceClient(inputs, "snapshot_policy_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.GetFilesystemSnapshotPolicy(fss.Context(), filestorage.GetFilesystemSnapshotPolicyRequest{FilesystemSnapshotPolicyId: &id})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}
	policy := summarisePolicy(&resp.FilesystemSnapshotPolicy)
	name := policy["display_name"]
	if name == "" {
		name = policy["id"]
	}
	return fss.Result(fmt.Sprintf("Snapshot policy %q is %s (%d schedule(s))", name, policy["lifecycle_state"], len(resp.Schedules)), map[string]interface{}{
		"snapshot_policy": policy, "id": policy["id"], "lifecycle_state": policy["lifecycle_state"],
	}), nil
}
