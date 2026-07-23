// Package oracle_filestorage_replication_target_get reads one replication target by OCID.
// The replication target is the destination side of a File Storage replication — it lives
// in the target region and applies the delta snapshots to the target file system.
package oracle_filestorage_replication_target_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	fss "flomation.app/automate/executor/actions/oracle/filestorage"

	filestorage "github.com/oracle/oci-go-sdk/v65/filestorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI File Storage: Get Replication Target"
	Description  = "Fetch a single Oracle Cloud File Storage replication target by OCID — the destination side of a replication, showing its source/target file systems, delta status and recovery point."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+copy"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the replication-target picker)"},
	{Name: "replication_target_ocid", Type: core.ConnectionTypeString, Label: "Replication Target OCID", Placeholder: "ocid1.replicationtarget.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "replication_target", Type: core.ConnectionTypeObject, Label: "Replication Target"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Replication Target OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := fss.ResourceClient(inputs, "replication_target_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.GetReplicationTarget(fss.Context(), filestorage.GetReplicationTargetRequest{ReplicationTargetId: &id})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}
	t := resp.ReplicationTarget
	target := map[string]interface{}{
		"id":                  fss.Str(t.Id),
		"display_name":        fss.Str(t.DisplayName),
		"compartment_id":      fss.Str(t.CompartmentId),
		"availability_domain": fss.Str(t.AvailabilityDomain),
		"lifecycle_state":     string(t.LifecycleState),
		"lifecycle_details":   fss.Str(t.LifecycleDetails),
		"replication_id":      fss.Str(t.ReplicationId),
		"source_id":           fss.Str(t.SourceId),
		"target_id":           fss.Str(t.TargetId),
		"last_snapshot_id":    fss.Str(t.LastSnapshotId),
		"delta_status":        string(t.DeltaStatus),
		"delta_progress":      fss.Int64OrNil(t.DeltaProgress),
		"recovery_point_time": fss.FormatTime(t.RecoveryPointTime),
		"time_created":        fss.FormatTime(t.TimeCreated),
	}
	return fss.Result(fmt.Sprintf("Replication target %q is %s", target["display_name"], target["lifecycle_state"]), map[string]interface{}{
		"replication_target": target, "id": target["id"], "lifecycle_state": target["lifecycle_state"],
	}), nil
}
