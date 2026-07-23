// Package oracle_exadata_cloud_exadata_infrastructure_update updates a cloud Exadata
// infrastructure — rename it or scale its compute/storage server counts. This is
// asynchronous: it returns the resource in an UPDATING state plus a work-request id;
// poll the Get action until the lifecycle state settles.
package oracle_exadata_cloud_exadata_infrastructure_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	exa "flomation.app/automate/executor/actions/oracle/exadata"

	db "github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Exadata: Update Cloud Exadata Infrastructure"
	Description  = "Update a cloud Exadata infrastructure by OCID — rename it, or scale its compute and storage server counts."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+microchip"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "cloud_exadata_infrastructure_ocid", Type: core.ConnectionTypeString, Label: "Cloud Exadata Infrastructure OCID", Placeholder: "ocid1.cloudexadatainfrastructure.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New user-friendly name (optional)"},
	{Name: "compute_count", Type: core.ConnectionTypeString, Label: "Compute Server Count", Placeholder: "Scale the number of compute servers (optional)"},
	{Name: "storage_count", Type: core.ConnectionTypeString, Label: "Storage Server Count", Placeholder: "Scale the number of storage servers (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "infrastructure", Type: core.ConnectionTypeObject, Label: "Infrastructure"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Infrastructure OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := exa.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	id, err := exa.RequiredString("cloud_exadata_infrastructure_ocid", inputs)
	if err != nil {
		return exa.ErrorResult(err.Error()), nil
	}

	// Each field is only sent when the operator supplies it. A blank input leaves the field
	// nil, which the SDK strips from the request body (UpdateCloudExadataInfrastructureDetails
	// tags them mandatory:false), so OCI leaves that attribute UNCHANGED — this is a partial
	// update, not a full replace. OptionalInt's ok flag distinguishes a blank count (unchanged)
	// from an explicit value, so a deliberately-typed number (even 0) is sent as given.
	details := db.UpdateCloudExadataInfrastructureDetails{}
	if name := exa.OptionalString("display_name", inputs); name != "" {
		details.DisplayName = &name
	}
	if n, ok, err := exa.OptionalInt("compute_count", inputs); err != nil {
		return exa.ErrorResult(err.Error()), nil
	} else if ok {
		details.ComputeCount = &n
	}
	if n, ok, err := exa.OptionalInt("storage_count", inputs); err != nil {
		return exa.ErrorResult(err.Error()), nil
	} else if ok {
		details.StorageCount = &n
	}

	resp, err := client.UpdateCloudExadataInfrastructure(exa.Context(), db.UpdateCloudExadataInfrastructureRequest{
		CloudExadataInfrastructureId:            &id,
		UpdateCloudExadataInfrastructureDetails: details,
	})
	if err != nil {
		return exa.ErrorResult(auth.OCIError(err)), nil
	}
	infra := exa.SummariseCloudExadataInfrastructure(&resp.CloudExadataInfrastructure)
	return exa.Result(fmt.Sprintf("Updating infrastructure %q — now %s", infra["display_name"], infra["lifecycle_state"]), map[string]interface{}{
		"infrastructure":  infra,
		"id":              infra["id"],
		"lifecycle_state": infra["lifecycle_state"],
		"work_request_id": exa.Str(resp.OpcWorkRequestId),
	}), nil
}
