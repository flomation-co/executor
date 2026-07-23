// Package oracle_filestorage_snapshot_update re-tags a snapshot and/or sets its
// expiration time. The snapshot name is immutable, so only free-form tags and the
// expiration time can be changed.
package oracle_filestorage_snapshot_update

import (
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	fss "flomation.app/automate/executor/actions/oracle/filestorage"

	"github.com/oracle/oci-go-sdk/v65/common"
	filestorage "github.com/oracle/oci-go-sdk/v65/filestorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI File Storage: Update Snapshot"
	Description  = "Update an Oracle Cloud file system snapshot's free-form tags and/or expiration time (the snapshot name is immutable). Only the fields you supply are changed; the rest are left as-is."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+clock-rotate-left"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the snapshot picker)"},
	{Name: "snapshot_ocid", Type: core.ConnectionTypeString, Label: "Snapshot OCID", Placeholder: "ocid1.snapshot.oc1..aaaa…", Required: true},
	{Name: "expiration_time", Type: core.ConnectionTypeString, Label: "Expiration Time (RFC3339)", Placeholder: "e.g. 2026-12-31T00:00:00Z — when to auto-delete (leave blank to keep)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Free-form Tags (JSON)", Placeholder: `e.g. {"env":"prod"} — replaces all free-form tags; leave blank to keep`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "snapshot", Type: core.ConnectionTypeObject, Label: "Snapshot"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Snapshot OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := fss.ResourceClient(inputs, "snapshot_ocid")
	if errResult != nil {
		return errResult, nil
	}

	details := filestorage.UpdateSnapshotDetails{}
	changed := false

	if v := strings.TrimSpace(fss.OptionalString("expiration_time", inputs)); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return fss.ErrorResult(fmt.Sprintf("invalid expiration time %q: expected RFC3339 (e.g. 2026-12-31T00:00:00Z)", v)), nil
		}
		details.ExpirationTime = &common.SDKTime{Time: t}
		changed = true
	}

	tags, err := fss.FreeformTags("tags", inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	if tags != nil {
		details.FreeformTags = tags
		changed = true
	}

	if !changed {
		return fss.ErrorResult("nothing to update — supply an expiration time and/or free-form tags"), nil
	}

	resp, err := client.UpdateSnapshot(fss.Context(), filestorage.UpdateSnapshotRequest{
		SnapshotId:            &id,
		UpdateSnapshotDetails: details,
	})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}

	snap := fss.SummariseSnapshot(&resp.Snapshot)
	return fss.Result(fmt.Sprintf("Updated snapshot %q (%s)", snap["name"], snap["lifecycle_state"]), map[string]interface{}{
		"snapshot": snap, "id": snap["id"], "lifecycle_state": snap["lifecycle_state"],
	}), nil
}
