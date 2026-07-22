// Package oracle_filestorage_replication_get reads one cross-region replication by OCID.
package oracle_filestorage_replication_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	fss "flomation.app/automate/executor/actions/oracle/filestorage"

	filestorage "github.com/oracle/oci-go-sdk/v65/filestorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI File Storage: Get Replication"
	Description  = "Fetch a single Oracle Cloud File Storage replication by OCID — its source and target file systems, replication interval, delta status and lifecycle state."
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "replication", Type: core.ConnectionTypeObject, Label: "Replication"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Replication OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := fss.ResourceClient(inputs, "replication_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.GetReplication(fss.Context(), filestorage.GetReplicationRequest{ReplicationId: &id})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}
	r := resp.Replication
	replication := map[string]interface{}{
		"id":                    fss.Str(r.Id),
		"display_name":          fss.Str(r.DisplayName),
		"compartment_id":        fss.Str(r.CompartmentId),
		"availability_domain":   fss.Str(r.AvailabilityDomain),
		"lifecycle_state":       string(r.LifecycleState),
		"lifecycle_details":     fss.Str(r.LifecycleDetails),
		"source_id":             fss.Str(r.SourceId),
		"target_id":             fss.Str(r.TargetId),
		"replication_target_id": fss.Str(r.ReplicationTargetId),
		"replication_interval":  fss.Int64OrNil(r.ReplicationInterval),
		"last_snapshot_id":      fss.Str(r.LastSnapshotId),
		"recovery_point_time":   fss.FormatTime(r.RecoveryPointTime),
		"delta_status":          string(r.DeltaStatus),
		"delta_progress":        fss.Int64OrNil(r.DeltaProgress),
		"freeform_tags":         r.FreeformTags,
		"time_created":          fss.FormatTime(r.TimeCreated),
	}
	return fss.Result(fmt.Sprintf("Replication %q is %s", replication["display_name"], replication["lifecycle_state"]), map[string]interface{}{
		"replication": replication, "id": replication["id"], "lifecycle_state": replication["lifecycle_state"],
	}), nil
}
