// Package oracle_filestorage_replication_create creates a cross-region File Storage
// replication between a source and a target file system (and its replication target).
package oracle_filestorage_replication_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	fss "flomation.app/automate/executor/actions/oracle/filestorage"

	filestorage "github.com/oracle/oci-go-sdk/v65/filestorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI File Storage: Create Replication"
	Description  = "Create an Oracle Cloud File Storage replication between a source file system and a target file system in another region or availability domain — the replication periodically copies snapshot deltas to keep the target current."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (holds the new replication)", Required: true},
	{Name: "source_file_system_ocid", Type: core.ConnectionTypeString, Label: "Source File System OCID", Placeholder: "ocid1.filesystem.oc1..aaaa… (the file system to replicate FROM)", Required: true},
	{Name: "target_file_system_ocid", Type: core.ConnectionTypeString, Label: "Target File System OCID", Placeholder: "ocid1.filesystem.oc1..aaaa… (an unexported file system in another region/AD to replicate TO)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "e.g. london-to-phoenix (optional)"},
	{Name: "replication_interval_minutes", Type: core.ConnectionTypeString, Label: "Replication Interval (minutes)", Placeholder: "Minutes between replication snapshots, e.g. 60 (optional)"},
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
	auth, client, errResult := fss.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	sourceID, err := fss.RequiredString("source_file_system_ocid", inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	targetID, err := fss.RequiredString("target_file_system_ocid", inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}

	details := filestorage.CreateReplicationDetails{
		CompartmentId: &compartment,
		SourceId:      &sourceID,
		TargetId:      &targetID,
	}
	if v := fss.OptionalString("display_name", inputs); v != "" {
		details.DisplayName = &v
	}
	if n, ok, err := fss.OptionalInt("replication_interval_minutes", inputs); err != nil {
		return fss.ErrorResult(err.Error()), nil
	} else if ok {
		interval := int64(n)
		details.ReplicationInterval = &interval
	}

	resp, err := client.CreateReplication(fss.Context(), filestorage.CreateReplicationRequest{CreateReplicationDetails: details})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}

	r := resp.Replication
	rep := map[string]interface{}{
		"id":                   fss.Str(r.Id),
		"display_name":         fss.Str(r.DisplayName),
		"compartment_id":       fss.Str(r.CompartmentId),
		"source_id":            fss.Str(r.SourceId),
		"target_id":            fss.Str(r.TargetId),
		"lifecycle_state":      string(r.LifecycleState),
		"replication_interval": fss.Int64OrNil(r.ReplicationInterval),
		"time_created":         fss.FormatTime(r.TimeCreated),
	}
	return fss.Result(
		fmt.Sprintf("Replication %q is %s — poll Get Replication until ACTIVE", rep["display_name"], rep["lifecycle_state"]),
		map[string]interface{}{
			"replication":     rep,
			"id":              rep["id"],
			"lifecycle_state": rep["lifecycle_state"],
		},
	), nil
}
